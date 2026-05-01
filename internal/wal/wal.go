package wal

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
)

const (
	FilePrefix = "wal_"
	FileSuffix = ".log"
)

type WAL struct {
	Dir           string
	ActiveSegment *Segment
	BlockSize     int
	MaxBlocks     int
	NextSegmentID uint64
	NextTxnID     uint64
	BM            *block.BlockManager
	Manifest      *WALManifest
}

type TxnOp struct {
	SeqId     uint64
	ExpiresAt int64
	OpType    enums.OpType
	Key       []byte
	Value     []byte
}

func OpenWAL() (*WAL, error) {
	settings := config.GetSettings()
	dir := path.Join(settings.SavePath, settings.WAL.SaveDirectory)
	blockSize := settings.WAL.BlockSize

	if err := block.EnsureDir(dir); err != nil {
		return nil, err
	}

	bm := block.NewBlockManager(blockSize)

	w := &WAL{
		Dir:           dir,
		BlockSize:     blockSize,
		MaxBlocks:     settings.WAL.MaxBlocks,
		NextSegmentID: 1,
		NextTxnID:     1,
		BM:            bm,
	}

	manifest, err := NewWALManifest(dir)
	if err != nil {
		return nil, err
	}
	w.Manifest = manifest

	if len(w.Manifest.Segments) == 0 {
		firstPath := w.SegmentPath(1)
		seg, err := OpenSegment(1, firstPath, w.MaxBlocks, w.BM)
		if err != nil {
			return nil, err
		}
		w.ActiveSegment = seg
		w.NextSegmentID = 2

		if err := w.Manifest.AddSegment(1); err != nil {
			return nil, err
		}
		return w, nil
	}

	segs := w.Manifest.SortedSegments()
	last := segs[len(segs)-1]

	lastPath := w.SegmentPath(last.SegmentID)
	seg, err := OpenSegment(last.SegmentID, lastPath, w.MaxBlocks, w.BM)
	if err != nil {
		return nil, err
	}

	w.ActiveSegment = seg
	w.NextSegmentID = last.SegmentID + 1

	if err := w.InitNextTxnID(); err != nil {
		return nil, err
	}

	if w.ActiveSegment.IsFull() {
		if err := w.RotateSegment(); err != nil {
			return nil, err
		}
	}

	return w, nil
}

func (w *WAL) Append(r Record) error {
	if w == nil {
		return fmt.Errorf("wal is nil")
	}
	if w.ActiveSegment == nil {
		return fmt.Errorf("active segment is nil")
	}

	wr := WALRecord{
		FragType:  FULL,
		RecType:   SINGLE,
		TxnID:     0,
		KeySize:   uint64(len(r.Key)),
		ValueSize: uint64(len(r.Value)),
		Record:    r,
	}

	return w.AppendWALRecord(wr)
}

func (w *WAL) Put(key []byte, value []byte, seqId uint64, opType enums.OpType) {
	r := Record{
		ExpiresAt: 0,
		OpType:    opType,
		SeqId:     seqId,
		Key:       key,
		Value:     value,
	}
	err := w.Append(r)
	if err != nil {
		panic(err)
	}
}

func (w *WAL) PutWithTTL(key []byte, value []byte, seqId uint64, opType enums.OpType, ttl int64) {
	r := Record{
		ExpiresAt: ttl,
		OpType:    opType,
		SeqId:     seqId,
		Key:       key,
		Value:     value,
	}
	err := w.Append(r)
	if err != nil {
		panic(err)
	}
}

func (w *WAL) AppendWALRecord(wr WALRecord) error {
	if w == nil {
		return fmt.Errorf("wal is nil")
	}
	if w.ActiveSegment == nil {
		return fmt.Errorf("active segment is nil")
	}

	err := w.ActiveSegment.AppendWALRecord(wr)
	if err != nil {
		if !w.ActiveSegment.IsFull() {
			return err
		}

		err = w.RotateSegment()
		if err != nil {
			return err
		}

		err = w.ActiveSegment.AppendWALRecord(wr)
		if err != nil {
			return err
		}
	}

	return w.Sync()
}

func (w *WAL) BatchWrite(ops []TxnOp) error {
	if w == nil {
		return fmt.Errorf("wal is nil")
	}
	if w.ActiveSegment == nil {
		return fmt.Errorf("active segment is nil")
	}
	if len(ops) == 0 {
		return nil
	}

	txnID := w.NextTxnID
	w.NextTxnID++

	startRecord := WALRecord{
		FragType:  FULL,
		RecType:   START,
		TxnID:     txnID,
		KeySize:   0,
		ValueSize: 0,
		Record: Record{
			SeqId:     ops[0].SeqId,
			ExpiresAt: ops[0].ExpiresAt,
			OpType:    enums.OpTypePut,
			Key:       nil,
			Value:     nil,
		},
	}

	err := w.AppendWALRecord(startRecord)
	if err != nil {
		return err
	}

	for _, op := range ops {

		txRecord := WALRecord{
			FragType:  FULL,
			RecType:   TRANSACTION,
			TxnID:     txnID,
			KeySize:   uint64(len(op.Key)),
			ValueSize: uint64(len(op.Value)),
			Record: Record{
				SeqId:     op.SeqId,
				ExpiresAt: op.ExpiresAt,
				OpType:    op.OpType,
				Key:       op.Key,
				Value:     op.Value,
			},
		}

		err = w.AppendWALRecord(txRecord)
		if err != nil {
			return err
		}
	}

	commitRecord := WALRecord{
		FragType:  FULL,
		RecType:   COMMIT,
		TxnID:     txnID,
		KeySize:   0,
		ValueSize: 0,
		Record: Record{
			SeqId:     ops[len(ops)-1].SeqId,
			ExpiresAt: ops[len(ops)-1].ExpiresAt,
			OpType:    enums.OpTypePut,
			Key:       nil,
			Value:     nil,
		},
	}

	err = w.AppendWALRecord(commitRecord)
	if err != nil {
		return err
	}

	return nil
}

// doesn't call memtable just return list of records
func (w *WAL) Recover() ([]Record, error) {
	frags, err := w.ReadAllFragments()
	if err != nil {
		return nil, err
	}

	joined, err := JoinFragments(frags)
	if err != nil {
		return nil, err
	}

	records, err := ApplyTransactions(joined)
	if err != nil {
		return nil, err
	}

	return records, nil
}

func (w *WAL) ReadAllFragments() ([]WALRecord, error) {
	if w == nil {
		return nil, fmt.Errorf("wal is nil")
	}
	if w.Manifest == nil {
		return nil, fmt.Errorf("manifest is nil")
	}

	segments := w.Manifest.SortedSegments()
	all := make([]WALRecord, 0)

	for _, meta := range segments {
		if meta.ValidFromBlock >= uint64(w.MaxBlocks) {
			continue
		}

		seg, err := OpenSegment(
			meta.SegmentID,
			w.SegmentPath(meta.SegmentID),
			w.MaxBlocks,
			w.BM,
		)
		if err != nil {
			return nil, err
		}

		recs, err := seg.ReadAllRecordsFromBlock(uint32(meta.ValidFromBlock))
		if err != nil {
			return nil, err
		}

		for _, rec := range recs {
			if rec.Record.SeqId <= meta.FlushedUpToSeqID {
				continue
			}

			all = append(all, rec)
		}
	}

	return all, nil
}

func JoinFragments(frags []WALRecord) ([]WALRecord, error) {
	records := make([]WALRecord, 0)

	var current *WALRecord
	inFragment := false

	for _, frag := range frags {
		switch frag.FragType {
		case FULL:
			if inFragment {
				return records, nil
			}
			records = append(records, frag)

		case FIRST:
			if inFragment {
				return records, nil
			}

			rec := WALRecord{
				FragType:  FULL,
				RecType:   frag.RecType,
				TxnID:     frag.TxnID,
				KeySize:   frag.KeySize,
				ValueSize: frag.ValueSize,
				Record: Record{
					SeqId:     frag.Record.SeqId,
					ExpiresAt: frag.Record.ExpiresAt,
					OpType:    frag.Record.OpType,
					Key:       append([]byte(nil), frag.Record.Key...),
					Value:     append([]byte(nil), frag.Record.Value...),
				},
			}

			current = &rec
			inFragment = true

		case MIDDLE:
			if !inFragment || current == nil {
				return records, nil
			}

			current.Record.Key = append(current.Record.Key, frag.Record.Key...)
			current.Record.Value = append(current.Record.Value, frag.Record.Value...)

		case LAST:
			if !inFragment || current == nil {
				return records, nil
			}

			current.Record.Key = append(current.Record.Key, frag.Record.Key...)
			current.Record.Value = append(current.Record.Value, frag.Record.Value...)
			current.KeySize = uint64(len(current.Record.Key))
			current.ValueSize = uint64(len(current.Record.Value))

			records = append(records, *current)
			current = nil
			inFragment = false

		default:
			return records, nil
		}
	}

	if inFragment {
		return records, nil
	}

	return records, nil
}

func ApplyTransactions(records []WALRecord) ([]Record, error) {
	result := make([]Record, 0)

	pending := make(map[uint64][]Record)
	started := make(map[uint64]bool)

	for _, rec := range records {
		switch rec.RecType {
		case SINGLE:
			result = append(result, rec.Record)

		case START:
			started[rec.TxnID] = true
			pending[rec.TxnID] = make([]Record, 0)

		case TRANSACTION:
			if started[rec.TxnID] {
				pending[rec.TxnID] = append(pending[rec.TxnID], rec.Record)
			}

		case COMMIT:
			if started[rec.TxnID] {
				result = append(result, pending[rec.TxnID]...)
				delete(started, rec.TxnID)
				delete(pending, rec.TxnID)
			}
		}
	}

	return result, nil
}

func (w *WAL) MemtableFlushed(sequenceId uint64) error {
	if w == nil {
		return fmt.Errorf("wal is nil")
	}
	if w.Manifest == nil {
		return fmt.Errorf("manifest is nil")
	}
	if sequenceId < 0 {
		return fmt.Errorf("sequenceId must be non-negative")
	}

	target := uint64(sequenceId)
	segments := w.Manifest.SortedSegments()

	var foundSegmentID uint64
	var foundBlockID uint64
	found := false

	for _, meta := range segments {
		if meta.ValidFromBlock >= uint64(w.MaxBlocks) {
			continue
		}

		if meta.FlushedUpToSeqID >= target {
			continue
		}

		seg, err := OpenSegment(
			meta.SegmentID,
			w.SegmentPath(meta.SegmentID),
			w.MaxBlocks,
			w.BM,
		)
		if err != nil {
			return err
		}

		for b := uint32(meta.ValidFromBlock); b < uint32(w.MaxBlocks); b++ {
			blockData, err := seg.ReadBlock(b)
			if err != nil {
				return err
			}

			recs, err := ReadBlockRecords(blockData)
			if err != nil {
				return err
			}

			if len(recs) == 0 {
				break
			}

			for _, rec := range recs {
				if rec.Record.SeqId <= target {
					foundSegmentID = meta.SegmentID
					foundBlockID = uint64(b)
					found = true
				}
			}
		}
	}

	if !found {
		return fmt.Errorf("sequenceId %d not found in WAL", sequenceId)
	}

	if err := w.Manifest.SetValidFromBlock(foundSegmentID, foundBlockID); err != nil {
		return err
	}

	if err := w.Manifest.SetFlushedUpToSeqID(foundSegmentID, target); err != nil {
		return err
	}

	for _, meta := range segments {
		if meta.SegmentID >= foundSegmentID {
			continue
		}

		if err := w.Manifest.SetValidFromBlock(
			meta.SegmentID,
			uint64(w.MaxBlocks),
		); err != nil {
			return err
		}

		if err := w.Manifest.SetFlushedUpToSeqID(
			meta.SegmentID,
			target,
		); err != nil {
			return err
		}
	}

	if foundSegmentID > 1 {
		if err := w.SetLowWatermark(foundSegmentID - 1); err != nil {
			return err
		}
	}

	return nil
}

func (w *WAL) SetLowWatermark(segmentID uint64) error {
	if w == nil {
		return fmt.Errorf("wal is nil")
	}
	if w.ActiveSegment != nil && segmentID >= w.ActiveSegment.ID {
		return fmt.Errorf("cannot set low watermark to active or future segment")
	}
	if err := w.Manifest.SetLowWatermark(segmentID); err != nil {
		return err
	}
	return w.DeleteOldSegments()
}

func (w *WAL) RotateSegment() error {
	if err := w.ActiveSegment.Sync(); err != nil {
		return err
	}
	newPath := w.SegmentPath(w.NextSegmentID)
	seg, err := OpenSegment(w.NextSegmentID, newPath, w.MaxBlocks, w.BM)
	if err != nil {
		return err
	}
	if err := w.Manifest.AddSegment(w.NextSegmentID); err != nil {
		return err
	}
	w.ActiveSegment = seg
	w.NextSegmentID++
	return nil
}

func (w *WAL) Sync() error {
	if w == nil {
		return fmt.Errorf("wal is nil")
	}
	if w.ActiveSegment == nil {
		return fmt.Errorf("active segment is nil")
	}
	return w.ActiveSegment.Sync()
}

func (w *WAL) PrintAll() error { // func for debugging
	if w == nil {
		return fmt.Errorf("wal is nil")
	}
	if w.Manifest == nil {
		return fmt.Errorf("manifest is nil")
	}

	segments := w.Manifest.SortedSegments()

	for _, segMeta := range segments {
		s, err := OpenSegment(segMeta.SegmentID, w.SegmentPath(segMeta.SegmentID), w.MaxBlocks, w.BM)
		if err != nil {
			return err
		}

		fmt.Printf(
			"Segment %d (ValidFromBlock=%d, FlushedUpToSeqID=%d)\n",
			segMeta.SegmentID,
			segMeta.ValidFromBlock,
			segMeta.FlushedUpToSeqID,
		)

		for bindex := uint32(0); bindex < uint32(w.MaxBlocks); bindex++ {
			blockData, err := s.ReadBlock(bindex)
			if err != nil {
				return err
			}

			fmt.Printf("Block %d: %v\n", bindex, blockData)
		}
	}

	return nil
}

func ParseSegmentID(name string) (uint64, error) {
	base := strings.TrimPrefix(name, FilePrefix)
	base = strings.TrimSuffix(base, FileSuffix)

	id, err := strconv.ParseUint(base, 10, 64)
	if err != nil {
		return 0, err
	}
	return id, nil

}

func (w *WAL) SegmentPath(id uint64) string {
	filename := fmt.Sprintf("wal_%06d.log", id)
	return filepath.Join(w.Dir, filename)
}

func (w *WAL) InitNextTxnID() error {
	frags, err := w.ReadAllFragments()
	if err != nil {
		return err
	}

	var maxTxn uint64 = 0
	for _, frag := range frags {
		if frag.TxnID > maxTxn {
			maxTxn = frag.TxnID
		}
	}

	w.NextTxnID = maxTxn + 1
	if w.NextTxnID == 0 {
		w.NextTxnID = 1
	}

	return nil
}
func (w *WAL) DeleteOldSegments() error {
	if w == nil {
		return fmt.Errorf("wal is nil")
	}
	if w.Manifest == nil {
		return fmt.Errorf("manifest is nil")
	}

	segments := w.Manifest.SortedSegments()

	for _, segMeta := range segments {
		id := segMeta.SegmentID

		if id > w.Manifest.LowWatermark {
			continue
		}

		if w.ActiveSegment != nil && id == w.ActiveSegment.ID {
			continue
		}

		p := w.SegmentPath(id)

		if w.BM != nil {
			w.BM.InvalidateFile(p)
		}

		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}

		if err := w.Manifest.RemoveSegment(id); err != nil {
			return err
		}
	}

	return nil
}

func (w *WAL) ClearAll(createNewActive bool) error {
	if w == nil {
		return fmt.Errorf("wal is nil")
	}

	segments := w.Manifest.SortedSegments()
	for _, seg := range segments {
		p := w.SegmentPath(seg.SegmentID)
		if w.BM != nil {
			w.BM.InvalidateFile(p)
		}
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	w.Manifest.Segments = make([]WALManifestEntry, 0)
	w.Manifest.LowWatermark = 0
	if err := w.Manifest.Save(); err != nil {
		return err
	}

	w.ActiveSegment = nil
	w.NextSegmentID = 1
	w.NextTxnID = 1

	if createNewActive {
		firstPath := w.SegmentPath(1)
		seg, err := OpenSegment(1, firstPath, w.MaxBlocks, w.BM)
		if err != nil {
			return err
		}
		w.ActiveSegment = seg
		w.NextSegmentID = 2
		return w.Manifest.AddSegment(1)
	}

	return nil
}
