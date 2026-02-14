package wal

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
)

type WAL struct {
	cfg *config.WALConfig

	dir       string
	blockSize int
	bm        *block.BlockManager

	mu sync.Mutex

	segment            *Segment
	segmentID          uint64
	blockIndex         uint32
	curBlock           *Block
	maxBlocksInSegment uint32 // derived from cfg.SegmentSize and blockSize

	// sync policy
	syncInterval time.Duration
	lastSync     time.Time

	// cleanup policy (low-water mark)
	lowWaterMark uint64

	// --- transactional state (START/OP/COMMIT) ---
	nextTxnID uint64
	inTxn     bool
	curTxnID  uint64
}

func NewWAL(cfg *config.WALConfig, dir string, bm *block.BlockManager) (*WAL, error) {
	if cfg == nil {
		return nil, fmt.Errorf("wal: nil config")
	}
	if bm == nil {
		return nil, fmt.Errorf("wal: nil block manager")
	}
	if cfg.BlockSize <= 0 {
		return nil, fmt.Errorf("wal: invalid blockSize")
	}
	if cfg.SegmentSize <= 0 {
		return nil, fmt.Errorf("wal: segment size must be positive")
	}

	// check if WAL block size matches BlockManager
	if bm.BlockSize() != cfg.BlockSize {
		return nil, fmt.Errorf("wal: block size mismatch (wal=%d, bm=%d)", cfg.BlockSize, bm.BlockSize())
	}

	if cfg.SegmentSize < cfg.BlockSize {
		return nil, fmt.Errorf("wal: segment_size (%d) must be >= block_size (%d)", cfg.SegmentSize, cfg.BlockSize)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	w := &WAL{
		cfg:       cfg,
		dir:       dir,
		blockSize: cfg.BlockSize,
		bm:        bm,
		curBlock:  NewBlock(cfg.BlockSize),
		nextTxnID: 1,
	}

	blocks := cfg.SegmentSize / cfg.BlockSize
	if blocks < 1 {
		blocks = 1
	}
	if blocks > int(math.MaxUint32) {
		blocks = int(math.MaxUint32)
	}
	w.maxBlocksInSegment = uint32(blocks)

	// SyncInterval, miliseconds
	// if 0 => sync Flush/Close/CommitTxn.
	if cfg.SyncInterval > 0 {
		w.syncInterval = time.Duration(cfg.SyncInterval) * time.Millisecond
	} else {
		w.syncInterval = 0
	}
	w.lastSync = time.Now()

	// Start after last existing segment (no overwrite)
	existing, err := listSegmentIDs(dir)
	if err != nil {
		return nil, err
	}
	if len(existing) == 0 {
		w.segmentID = 1
	} else {
		w.segmentID = existing[len(existing)-1] + 1
	}

	if err := w.openSegment(w.segmentID); err != nil {
		return nil, err
	}

	return w, nil
}

func (w *WAL) openSegment(id uint64) error {
	filename := filepath.Join(w.dir, fmt.Sprintf("wal_%020d.log", id))
	w.segment = NewSegment(filename, id, w.bm)
	w.blockIndex = 0
	w.curBlock.Reset()

	// preallocation
	segBytes := int64(w.maxBlocksInSegment) * int64(w.blockSize)
	if err := w.segment.EnsureFixedSize(segBytes); err != nil {
		return err
	}

	return nil
}

func (w *WAL) maybeSyncLocked(force bool) error {
	if w.segment == nil {
		return nil
	}
	if force {
		if err := w.segment.Sync(); err != nil {
			return err
		}
		w.lastSync = time.Now()
		return nil
	}
	if w.syncInterval <= 0 {
		return nil
	}
	if time.Since(w.lastSync) >= w.syncInterval {
		if err := w.segment.Sync(); err != nil {
			return err
		}
		w.lastSync = time.Now()
	}
	return nil
}

func (w *WAL) flushCurrentBlockIfNeededLocked() error {
	if w.curBlock.Pos() == 0 {
		return nil
	}
	w.curBlock.PadToEnd()
	if err := w.segment.WriteBlock(w.blockIndex, w.curBlock.Bytes()); err != nil {
		return err
	}
	w.blockIndex++
	w.curBlock.Reset()

	// sync poslije flasa
	if err := w.maybeSyncLocked(false); err != nil {
		return err
	}

	if w.blockIndex >= w.maxBlocksInSegment {
		return w.rotateSegmentLocked()
	}
	return nil
}

func (w *WAL) rotateSegmentLocked() error {
	// flush partial
	if w.curBlock.Pos() > 0 {
		w.curBlock.PadToEnd()
		if err := w.segment.WriteBlock(w.blockIndex, w.curBlock.Bytes()); err != nil {
			return err
		}
		w.blockIndex++
		w.curBlock.Reset()
	}

	// force sync
	if err := w.maybeSyncLocked(true); err != nil {
		return err
	}

	w.segmentID++
	return w.openSegment(w.segmentID)
}

// appendRecordLocked writes a single WAL Record (already validated) into blocks/fragments.
// Caller must hold w.mu.
func (w *WAL) appendRecordLocked(r Record) error {
	if r.Timestamp == 0 {
		r.Timestamp = uint64(time.Now().UnixNano())
	}
	if w.segmentID > uint64(math.MaxUint32) {
		return fmt.Errorf("wal: segmentID overflow for fragment logNumber (segmentID=%d)", w.segmentID)
	}

	recBytes, err := Encode(r)
	if err != nil {
		return err
	}

	remaining := len(recBytes)
	offset := 0
	isFirst := true

	nextFragType := func(first bool, rem int, cap int) FragmentType {
		if first && rem <= cap {
			return FragFull
		}
		if first {
			return FragFirst
		}
		if rem <= cap {
			return FragLast
		}
		return FragMiddle
	}

	for remaining > 0 {
		if w.curBlock.Remaining() < FragmentHeaderSize {
			if err := w.flushCurrentBlockIfNeededLocked(); err != nil {
				return err
			}
		}

		payloadCap := w.curBlock.Remaining() - FragmentHeaderSize
		if payloadCap <= 0 {
			if err := w.flushCurrentBlockIfNeededLocked(); err != nil {
				return err
			}
			continue
		}

		chunkLen := remaining
		if chunkLen > payloadCap {
			chunkLen = payloadCap
		}

		ft := nextFragType(isFirst, remaining, payloadCap)
		payload := recBytes[offset : offset+chunkLen]

		frag, err := NewFragment(ft, uint32(w.segmentID), payload)
		if err != nil {
			return err
		}

		hdr := make([]byte, FragmentHeaderSize)
		if err := EncodeHeader(frag.Header, hdr); err != nil {
			return err
		}

		if err := w.curBlock.WriteBytes(hdr); err != nil {
			return err
		}
		if err := w.curBlock.WriteBytes(frag.Payload); err != nil {
			return err
		}

		offset += chunkLen
		remaining -= chunkLen
		isFirst = false

		if w.curBlock.Remaining() == 0 {
			if err := w.flushCurrentBlockIfNeededLocked(); err != nil {
				return err
			}
		}
	}

	return w.maybeSyncLocked(false)
}

func (w *WAL) Append(r Record) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.appendRecordLocked(r)
}

func (w *WAL) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.flushCurrentBlockIfNeededLocked(); err != nil {
		return err
	}
	// Flush durability boundary, force sync
	return w.maybeSyncLocked(true)
}

func (w *WAL) Close() error {
	return w.Flush()
}

// transactions

// BeginTxn appends a START record and returns txnID.
//
//	at most 1 open txn at a time (contiguous START..OP..COMMIT)
func (w *WAL) BeginTxn() (uint64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.inTxn {
		return 0, fmt.Errorf("wal: transaction already in progress (txnID=%d)", w.curTxnID)
	}

	txnID := w.nextTxnID
	w.nextTxnID++

	ts := uint64(time.Now().UnixNano())
	if err := w.appendRecordLocked(NewTxnStart(txnID, ts)); err != nil {
		return 0, err
	}

	w.inTxn = true
	w.curTxnID = txnID
	return txnID, nil
}

func (w *WAL) Put(txnID uint64, key, value []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.inTxn || w.curTxnID != txnID {
		return fmt.Errorf("wal: Put outside current transaction (got=%d, cur=%d, inTxn=%v)", txnID, w.curTxnID, w.inTxn)
	}
	ts := uint64(time.Now().UnixNano())
	return w.appendRecordLocked(NewTxnPut(txnID, ts, key, value))
}

func (w *WAL) Delete(txnID uint64, key []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.inTxn || w.curTxnID != txnID {
		return fmt.Errorf("wal: Delete outside current transaction (got=%d, cur=%d, inTxn=%v)", txnID, w.curTxnID, w.inTxn)
	}
	ts := uint64(time.Now().UnixNano())
	return w.appendRecordLocked(NewTxnDelete(txnID, ts, key))
}

func (w *WAL) DeleteRange(txnID uint64, start, end []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.inTxn || w.curTxnID != txnID {
		return fmt.Errorf("wal: DeleteRange outside current transaction (got=%d, cur=%d, inTxn=%v)", txnID, w.curTxnID, w.inTxn)
	}
	ts := uint64(time.Now().UnixNano())
	return w.appendRecordLocked(NewTxnRangeDelete(txnID, ts, start, end))
}

// CommitTxn appends COMMIT and makes it durable (fsync boundary).
func (w *WAL) CommitTxn(txnID uint64) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.inTxn || w.curTxnID != txnID {
		return fmt.Errorf("wal: CommitTxn outside current transaction (got=%d, cur=%d, inTxn=%v)", txnID, w.curTxnID, w.inTxn)
	}

	ts := uint64(time.Now().UnixNano())
	if err := w.appendRecordLocked(NewTxnCommit(txnID, ts)); err != nil {
		return err
	}

	// Ensure COMMIT is on disk: flush any partial block and fsync.
	if err := w.flushCurrentBlockIfNeededLocked(); err != nil {
		return err
	}
	if err := w.maybeSyncLocked(true); err != nil {
		return err
	}

	w.inTxn = false
	w.curTxnID = 0
	return nil
}

// Low-water mark API (OVO SE ZOVE POSLIJE SSTABLE FLASA U ENGINU)
func (w *WAL) SetLowWaterMark(segID uint64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if segID > w.lowWaterMark {
		w.lowWaterMark = segID
	}
}

// Cleanup deletes segments <= lowWaterMark
func (w *WAL) Cleanup() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.lowWaterMark == 0 {
		return nil
	}

	ids, err := listSegmentIDs(w.dir)
	if err != nil {
		return err
	}

	for _, id := range ids {
		if id > w.lowWaterMark {
			continue
		}
		if w.segment != nil && id == w.segmentID {
			continue
		}
		path := filepath.Join(w.dir, fmt.Sprintf("wal_%020d.log", id))
		_ = os.Remove(path)
	}

	// maxsegments check
	if w.cfg.MaxSegments > 0 {
		ids2, err := listSegmentIDs(w.dir)
		if err != nil {
			return err
		}
		if len(ids2) > w.cfg.MaxSegments {
			keepFrom := len(ids2) - w.cfg.MaxSegments
			for i := 0; i < keepFrom; i++ {
				id := ids2[i]
				if id > w.lowWaterMark {
					continue
				}
				if w.segment != nil && id == w.segmentID {
					continue
				}
				path := filepath.Join(w.dir, fmt.Sprintf("wal_%020d.log", id))
				_ = os.Remove(path)
			}
		}
	}

	return nil
}

// RecoverTxn replays WAL and applies ONLY committed transactions.
// applyOp gets called for OP records (PUT/DEL/RANGE_DEL) in commit order.
// Non-committed txn (missing COMMIT due to crash) is ignored.
func (w *WAL) RecoverTxn(applyOp func(Record) error) (int, error) {
	if applyOp == nil {
		return 0, fmt.Errorf("wal: applyOp callback is nil")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	segmentIDs, err := listSegmentIDs(w.dir)
	if err != nil {
		return 0, err
	}

	appliedOps := 0

	var (
		assembling bool
		buf        []byte

		openTxnID uint64
		openOps   []Record
	)

	resetAssembled := func() {
		assembling = false
		buf = nil
	}
	resetTxn := func() {
		openTxnID = 0
		openOps = nil
	}

	emitRecord := func(r Record) error {
		switch r.Type {
		case RecTxnStart:
			// If a txn is already open and we see a new START, its data corruption, stop
			if openTxnID != 0 {
				return ioStop
			}
			openTxnID = r.TxnID
			openOps = openOps[:0]
			return nil

		case RecTxnOp:
			if openTxnID == 0 || r.TxnID != openTxnID {
				return ioStop
			}
			openOps = append(openOps, r)
			return nil

		case RecTxnCommit:
			if openTxnID == 0 || r.TxnID != openTxnID {
				return ioStop
			}
			// Apply ops atomically at commit boundary
			for _, opRec := range openOps {
				if err := applyOp(opRec); err != nil {
					return err
				}
				appliedOps++
			}
			resetTxn()
			return nil

		default:
			return ioStop
		}
	}

	for _, sid := range segmentIDs {
		path := filepath.Join(w.dir, fmt.Sprintf("wal_%020d.log", sid))

		blocks, err := blocksInFile(path, w.blockSize)
		if err != nil {
			return appliedOps, err
		}

		seg := NewSegment(path, sid, w.bm)

		for bi := uint32(0); bi < blocks; bi++ {
			blockBytes, err := seg.ReadBlock(bi)
			if err != nil {
				resetAssembled()
				return appliedOps, nil
			}

			pos := 0
			for {
				if len(blockBytes)-pos < FragmentHeaderSize {
					break
				}
				headerBytes := blockBytes[pos : pos+FragmentHeaderSize]
				if isAllZero(headerBytes) {
					break
				}

				h, err := DecodeHeader(headerBytes)
				if err != nil {
					resetAssembled()
					return appliedOps, nil
				}
				pos += FragmentHeaderSize

				if len(blockBytes)-pos < int(h.Size) {
					resetAssembled()
					return appliedOps, nil
				}

				payload := blockBytes[pos : pos+int(h.Size)]
				pos += int(h.Size)

				f := Fragment{Header: h, Payload: payload}
				if err := f.Verify(); err != nil {
					resetAssembled()
					return appliedOps, nil
				}

				switch f.Header.Type {
				case FragFull:
					r, err := Decode(f.Payload)
					if err != nil {
						resetAssembled()
						return appliedOps, nil
					}
					if err := emitRecord(r); err != nil {
						if err == ioStop {
							return appliedOps, nil
						}
						return appliedOps, err
					}

				case FragFirst:
					assembling = true
					buf = append(buf[:0], f.Payload...)

				case FragMiddle:
					if !assembling {
						return appliedOps, nil
					}
					buf = append(buf, f.Payload...)

				case FragLast:
					if !assembling {
						return appliedOps, nil
					}
					buf = append(buf, f.Payload...)
					assembling = false

					r, err := Decode(buf)
					if err != nil {
						buf = nil
						return appliedOps, nil
					}
					buf = nil
					if err := emitRecord(r); err != nil {
						if err == ioStop {
							return appliedOps, nil
						}
						return appliedOps, err
					}

				default:
					resetAssembled()
					return appliedOps, nil
				}
			}
		}
	}

	// If crash happened mid-txn (no COMMIT), ignore openTxnID + openOp
	resetAssembled()
	return appliedOps, nil
}

var ioStop = fmt.Errorf("wal: stop")

func listSegmentIDs(dir string) ([]uint64, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var ids []uint64
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "wal_") || !strings.HasSuffix(name, ".log") {
			continue
		}
		numPart := strings.TrimSuffix(strings.TrimPrefix(name, "wal_"), ".log")
		u, err := strconv.ParseUint(numPart, 10, 64)
		if err != nil {
			continue
		}
		ids = append(ids, u)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

func blocksInFile(path string, blockSize int) (uint32, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	if info.IsDir() {
		return 0, fmt.Errorf("wal: segment path is a directory: %s", path)
	}
	if blockSize <= 0 {
		return 0, fmt.Errorf("wal: invalid blockSize")
	}
	size := info.Size()
	if size <= 0 {
		return 0, nil
	}
	full := size / int64(blockSize)
	if full > int64(^uint32(0)) {
		return ^uint32(0), nil
	}
	return uint32(full), nil
}

func isAllZero(b []byte) bool {
	for _, x := range b {
		if x != 0 {
			return false
		}
	}
	return true
}
