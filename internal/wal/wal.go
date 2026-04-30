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
)

// da li sece rekord po segmentima kako treba
// config.GetSettings().SavePath
// op type, expires at, sequence id, ttl

/*

 */

const (
	FilePrefix = "wal_"
	FileSuffix = ".log"
)

type WAL struct {
	Dir            string
	ActiveSegment  *Segment
	BlockSize      int
	MaxBlocks      int
	NextSegmentID  uint64
	NextSequenceID uint64
	NextTxnID      uint64
	LowWatermark   uint64
	BM             *block.BlockManager
}

type TxnOp struct {
	Timestamp uint64
	Tombstone bool
	Key       []byte
	Value     []byte
}

func OpenWAL() (*WAL, error) {
	settings := config.GetSettings()
	dir := path.Join(settings.SavePath, settings.WAL.SaveDirectory)
	blockSize := settings.WAL.BlockSize

	err := block.EnsureDir(dir)
	if err != nil {
		return nil, err
	}
	bm := block.NewBlockManager(blockSize)
	//print(dir)

	w := &WAL{
		Dir:            dir,
		BlockSize:      blockSize,
		MaxBlocks:      settings.WAL.MaxBlocks,
		NextSegmentID:  1,
		NextSequenceID: 1,
		NextTxnID:      1,
		LowWatermark:   0,
		BM:             bm,
	}

	entries, err := ListFiles(dir)
	if err != nil {
		return nil, err
	}
	maxID := uint64(0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, FilePrefix) || !strings.HasSuffix(name, FileSuffix) {
			continue
		}
		id, err := ParseSegmentID(name)
		if err != nil {
			return nil, err
		}
		if id > maxID {
			maxID = id
		}

	}

	if maxID == 0 { // id no log files
		firstPath := w.SegmentPath(w.NextSegmentID)
		seg, err := OpenSegment(w.NextSegmentID, firstPath, w.MaxBlocks, w.BM)
		if err != nil {
			return nil, err
		}
		w.ActiveSegment = seg
		w.NextSegmentID++

		return w, nil
	}

	// load last log file by id
	lastPath := w.SegmentPath(maxID)
	seg, err := OpenSegment(maxID, lastPath, w.MaxBlocks, w.BM)
	if err != nil {
		return nil, err
	}

	w.ActiveSegment = seg
	w.NextSegmentID = maxID + 1

	err = w.InitNextSequenceID()
	if err != nil {
		return nil, err
	}

	err = w.InitNextTxnID()
	if err != nil {
		return nil, err
	}

	if w.ActiveSegment.IsFull() {
		err = w.RotateSegment()
		if err != nil {
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

	sequenceID := w.NextSequenceID

	wr := WALRecord{
		FragType:   FULL,
		RecType:    SINGLE,
		TxnID:      0,
		SequenceID: sequenceID,
		KeySize:    uint64(len(r.Key)),
		ValueSize:  uint64(len(r.Value)),
		Record:     r,
	}

	err := w.AppendWALRecord(wr)
	if err != nil {
		return err
	}

	w.NextSequenceID++
	return nil
}

// umjesto timestamp, nula, Op type
func (w *WAL) Put(key []byte, value []byte) error {
	r := Record{
		Timestamp: 0, // TO DO
		Tombstone: false,
		Key:       key,
		Value:     value,
	}
	err := w.Append(r)
	if err != nil {
		return err
	}
	return nil
}

func (w *WAL) PutWithTTL(key []byte, value []byte, timestamp uint64) error {
	r := Record{
		Timestamp: timestamp,
		Tombstone: false,
		Key:       key,
		Value:     value,
	}
	err := w.Append(r)
	if err != nil {
		return err
	}
	return nil
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

	startSeq := w.NextSequenceID

	startRecord := WALRecord{
		FragType:   FULL,
		RecType:    START,
		TxnID:      txnID,
		SequenceID: startSeq,
		KeySize:    0,
		ValueSize:  0,
		Record: Record{
			Timestamp: ops[0].Timestamp,
			Tombstone: false,
			Key:       nil,
			Value:     nil,
		},
	}

	err := w.AppendWALRecord(startRecord)
	if err != nil {
		return err
	}
	w.NextSequenceID++

	for _, op := range ops {
		seq := w.NextSequenceID

		txRecord := WALRecord{
			FragType:   FULL,
			RecType:    TRANSACTION,
			TxnID:      txnID,
			SequenceID: seq,
			KeySize:    uint64(len(op.Key)),
			ValueSize:  uint64(len(op.Value)),
			Record: Record{
				Timestamp: op.Timestamp,
				Tombstone: op.Tombstone,
				Key:       op.Key,
				Value:     op.Value,
			},
		}

		err = w.AppendWALRecord(txRecord)
		if err != nil {
			return err
		}

		w.NextSequenceID++
	}

	commitSeq := w.NextSequenceID

	commitRecord := WALRecord{
		FragType:   FULL,
		RecType:    COMMIT,
		TxnID:      txnID,
		SequenceID: commitSeq,
		KeySize:    0,
		ValueSize:  0,
		Record: Record{
			Timestamp: ops[len(ops)-1].Timestamp,
			Tombstone: false,
			Key:       nil,
			Value:     nil,
		},
	}

	err = w.AppendWALRecord(commitRecord)
	if err != nil {
		return err
	}

	w.NextSequenceID++
	return nil
}

func (w *WAL) DeleteRange(keys [][]byte, timestamp uint64) error {
	ops := make([]TxnOp, 0, len(keys))
	for _, key := range keys {
		ops = append(ops, TxnOp{
			Timestamp: timestamp,
			Tombstone: true,
			Key:       key,
			Value:     nil,
		})
	}
	return w.BatchWrite(ops)
}

func (w *WAL) Delete(key []byte, timestamp uint64) error {
	r := Record{
		Timestamp: timestamp,
		Tombstone: true,
		Key:       key,
		Value:     nil,
	}
	err := w.Append(r)
	if err != nil {
		return err
	}
	return nil
}

func (w *WAL) Recover() ([]Record, error) { // doesnt call memtable just return list of records
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

	entries, err := ListFiles(w.Dir)
	if err != nil {
		return nil, err
	}

	type segInfo struct {
		id   uint64
		path string
	}

	segments := make([]segInfo, 0)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, FilePrefix) || !strings.HasSuffix(name, FileSuffix) {
			continue
		}

		id, err := ParseSegmentID(name)
		if err != nil {
			return nil, err
		}

		segments = append(segments, segInfo{
			id:   id,
			path: filepath.Join(w.Dir, name),
		})
	}

	for i := 0; i < len(segments); i++ {
		for j := i + 1; j < len(segments); j++ {
			if segments[j].id < segments[i].id {
				temp := segments[j]
				segments[j] = segments[i]
				segments[i] = temp
			}
		}
	}

	all := make([]WALRecord, 0)

	for _, segInfo := range segments {
		seg, err := OpenSegment(segInfo.id, segInfo.path, w.MaxBlocks, w.BM)
		if err != nil {
			return nil, err
		}

		recs, err := seg.ReadAllRecords()
		if err != nil {
			return nil, err
		}

		all = append(all, recs...)
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
				FragType:   FULL,
				RecType:    frag.RecType,
				TxnID:      frag.TxnID,
				SequenceID: frag.SequenceID,
				KeySize:    frag.KeySize,
				ValueSize:  frag.ValueSize,
				Record: Record{
					Timestamp: frag.Record.Timestamp,
					Tombstone: frag.Record.Tombstone,
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

func (w *WAL) SetLowWatermark(segmentID uint64) error {
	if w == nil {
		return fmt.Errorf("wal is nil")
	}

	if segmentID == 0 {
		return nil
	}

	if segmentID < w.LowWatermark {
		return fmt.Errorf("low watermark cannot move backwards")
	}

	if w.ActiveSegment != nil && segmentID >= w.ActiveSegment.ID {
		return fmt.Errorf("cannot set low watermark to active or future segment")
	}

	w.LowWatermark = segmentID
	return w.DeleteOldSegments()
}

func (w *WAL) RotateSegment() error {
	if w == nil {
		return fmt.Errorf("wal is nil")
	}
	if w.ActiveSegment == nil {
		return fmt.Errorf("active segment is nil")
	}
	err := w.ActiveSegment.Sync()
	if err != nil {
		return err
	}

	newPath := w.SegmentPath(w.NextSegmentID)
	seg, err := OpenSegment(w.NextSegmentID, newPath, w.MaxBlocks, w.BM)
	if err != nil {
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

	entries, err := ListFiles(w.Dir)
	if err != nil {
		return err
	}

	type segInfo struct {
		id   uint64
		path string
	}

	segments := make([]segInfo, 0)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasPrefix(name, FilePrefix) || !strings.HasSuffix(name, FileSuffix) {
			continue
		}

		id, err := ParseSegmentID(name)
		if err != nil {
			return err
		}

		segments = append(segments, segInfo{
			id:   id,
			path: filepath.Join(w.Dir, name),
		})
	}

	for i := 0; i < len(segments); i++ {
		for j := i + 1; j < len(segments); j++ {
			if segments[j].id < segments[i].id {
				segments[i], segments[j] = segments[j], segments[i]
			}
		}
	}

	for _, segInfo := range segments {
		s, err := OpenSegment(segInfo.id, segInfo.path, w.MaxBlocks, w.BM)
		if err != nil {
			return err
		}

		fmt.Println("Segment", segInfo.id)

		for bindex := 0; bindex < w.MaxBlocks; bindex++ {
			block, err := s.ReadBlock(uint32(bindex))
			if err != nil {
				return err
			}
			fmt.Println(block)
		}
	}

	return nil
}

func ParseSegmentID(name string) (uint64, error) {
	base := strings.TrimPrefix(name, FilePrefix)
	base = strings.TrimSuffix(base, FileSuffix)
	//fmt.Println(name, base)

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

// helper function, should go to block manager
func ListFiles(dir string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	return entries, err
}

func (w *WAL) InitNextSequenceID() error {
	frags, err := w.ReadAllFragments()
	if err != nil {
		return err
	}

	var maxSeq uint64 = 0
	for _, frag := range frags {
		if frag.SequenceID > maxSeq {
			maxSeq = frag.SequenceID
		}
	}

	w.NextSequenceID = maxSeq + 1
	if w.NextSequenceID == 0 {
		w.NextSequenceID = 1
	}

	return nil
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

	entries, err := ListFiles(w.Dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasPrefix(name, FilePrefix) || !strings.HasSuffix(name, FileSuffix) {
			continue
		}

		id, err := ParseSegmentID(name)
		if err != nil {
			return err
		}

		if id > w.LowWatermark {
			continue
		}

		if w.ActiveSegment != nil && id == w.ActiveSegment.ID {
			continue
		}

		path := filepath.Join(w.Dir, name)

		if w.BM != nil {
			w.BM.InvalidateFile(path)
		}

		err = os.Remove(path)
		if err != nil {
			return err
		}
	}

	return nil
}
