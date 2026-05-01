package wal

import (
	"bytes"
	"testing"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
)

func newTestWAL(t *testing.T, blockSize int, maxBlocks int) *WAL {
	t.Helper()

	dir := t.TempDir()
	bm := block.NewBlockManager(blockSize)

	manifest, err := NewWALManifest(dir)
	if err != nil {
		t.Fatalf("NewWALManifest failed: %v", err)
	}

	w := &WAL{
		Dir:           dir,
		BlockSize:     blockSize,
		MaxBlocks:     maxBlocks,
		NextSegmentID: 2,
		NextTxnID:     1,
		BM:            bm,
		Manifest:      manifest,
	}

	seg, err := OpenSegment(1, w.SegmentPath(1), maxBlocks, bm)
	if err != nil {
		t.Fatalf("OpenSegment failed: %v", err)
	}

	w.ActiveSegment = seg

	if err := manifest.AddSegment(1); err != nil {
		t.Fatalf("AddSegment failed: %v", err)
	}

	return w
}

func TestWALPutAndRecover(t *testing.T) {
	w := newTestWAL(t, KEY_START+64, 5)

	key := []byte("user:1")
	value := []byte("Kristian")

	w.Put(key, value, 1, enums.OpTypePut)

	records, err := w.Recover()
	if err != nil {
		t.Fatalf("Recover failed: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 recovered record, got %d", len(records))
	}

	got := records[0]

	if got.SeqId != 1 {
		t.Fatalf("expected SeqId 1, got %d", got.SeqId)
	}

	if got.OpType != enums.OpTypePut {
		t.Fatalf("expected OpTypePut, got %v", got.OpType)
	}

	if !bytes.Equal(got.Key, key) {
		t.Fatalf("expected key %q, got %q", key, got.Key)
	}

	if !bytes.Equal(got.Value, value) {
		t.Fatalf("expected value %q, got %q", value, got.Value)
	}
}

func TestWALRecoverFragmentedRecord(t *testing.T) {
	w := newTestWAL(t, KEY_START+10, 10)

	key := []byte("abcdefgh")
	value := []byte("1234567890123456789012345")

	w.Put(key, value, 1, enums.OpTypePut)

	records, err := w.Recover()
	if err != nil {
		t.Fatalf("Recover failed: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 recovered record, got %d", len(records))
	}

	got := records[0]

	if !bytes.Equal(got.Key, key) {
		t.Fatalf("expected key %q, got %q", key, got.Key)
	}

	if !bytes.Equal(got.Value, value) {
		t.Fatalf("expected value %q, got %q", value, got.Value)
	}
}

func TestWALBatchWriteCommittedTransactionIsRecovered(t *testing.T) {
	w := newTestWAL(t, KEY_START+64, 10)

	ops := []TxnOp{
		{
			SeqId:  1,
			OpType: enums.OpTypePut,
			Key:    []byte("k1"),
			Value:  []byte("v1"),
		},
		{
			SeqId:  2,
			OpType: enums.OpTypePut,
			Key:    []byte("k2"),
			Value:  []byte("v2"),
		},
	}

	if err := w.BatchWrite(ops); err != nil {
		t.Fatalf("BatchWrite failed: %v", err)
	}

	records, err := w.Recover()
	if err != nil {
		t.Fatalf("Recover failed: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 recovered records, got %d", len(records))
	}

	if records[0].SeqId != 1 {
		t.Fatalf("expected first SeqId 1, got %d", records[0].SeqId)
	}

	if records[1].SeqId != 2 {
		t.Fatalf("expected second SeqId 2, got %d", records[1].SeqId)
	}

	if !bytes.Equal(records[0].Key, []byte("k1")) {
		t.Fatalf("expected first key k1, got %q", records[0].Key)
	}

	if !bytes.Equal(records[0].Value, []byte("v1")) {
		t.Fatalf("expected first value v1, got %q", records[0].Value)
	}

	if !bytes.Equal(records[1].Key, []byte("k2")) {
		t.Fatalf("expected second key k2, got %q", records[1].Key)
	}

	if !bytes.Equal(records[1].Value, []byte("v2")) {
		t.Fatalf("expected second value v2, got %q", records[1].Value)
	}
}

func TestWALIncompleteTransactionIsIgnored(t *testing.T) {
	w := newTestWAL(t, KEY_START+64, 10)

	txnID := uint64(1)

	start := WALRecord{
		FragType: FULL,
		RecType:  START,
		TxnID:    txnID,
		Record: Record{
			SeqId:  1,
			OpType: enums.OpTypePut,
		},
	}

	transaction := WALRecord{
		FragType: FULL,
		RecType:  TRANSACTION,
		TxnID:    txnID,
		Record: Record{
			SeqId:  2,
			OpType: enums.OpTypePut,
			Key:    []byte("uncommitted"),
			Value:  []byte("value"),
		},
	}

	if err := w.AppendWALRecord(start); err != nil {
		t.Fatalf("append START failed: %v", err)
	}

	if err := w.AppendWALRecord(transaction); err != nil {
		t.Fatalf("append TRANSACTION failed: %v", err)
	}

	records, err := w.Recover()
	if err != nil {
		t.Fatalf("Recover failed: %v", err)
	}

	if len(records) != 0 {
		t.Fatalf("expected incomplete transaction to be ignored, got %d records", len(records))
	}
}

func TestWALMixedSingleAndCommittedTransactionRecover(t *testing.T) {
	w := newTestWAL(t, KEY_START+64, 10)

	w.Put([]byte("single"), []byte("value"), 1, enums.OpTypePut)

	ops := []TxnOp{
		{
			SeqId:  2,
			OpType: enums.OpTypePut,
			Key:    []byte("tx1"),
			Value:  []byte("v1"),
		},
		{
			SeqId:  3,
			OpType: enums.OpTypePut,
			Key:    []byte("tx2"),
			Value:  []byte("v2"),
		},
	}

	if err := w.BatchWrite(ops); err != nil {
		t.Fatalf("BatchWrite failed: %v", err)
	}

	records, err := w.Recover()
	if err != nil {
		t.Fatalf("Recover failed: %v", err)
	}

	if len(records) != 3 {
		t.Fatalf("expected 3 recovered records, got %d", len(records))
	}

	expectedKeys := [][]byte{
		[]byte("single"),
		[]byte("tx1"),
		[]byte("tx2"),
	}

	for i := range expectedKeys {
		if !bytes.Equal(records[i].Key, expectedKeys[i]) {
			t.Fatalf("record %d: expected key %q, got %q", i, expectedKeys[i], records[i].Key)
		}
	}
}

func TestWALRotatesSegmentWhenActiveSegmentIsFull(t *testing.T) {
	w := newTestWAL(t, KEY_START+5, 2)

	w.Put([]byte("a"), []byte("1111"), 1, enums.OpTypePut)
	w.Put([]byte("b"), []byte("2222"), 2, enums.OpTypePut)
	w.Put([]byte("c"), []byte("3333"), 3, enums.OpTypePut)

	if w.ActiveSegment.ID != 2 {
		t.Fatalf("expected active segment ID 2 after rotation, got %d", w.ActiveSegment.ID)
	}

	if len(w.Manifest.Segments) != 2 {
		t.Fatalf("expected 2 manifest segments, got %d", len(w.Manifest.Segments))
	}

	records, err := w.Recover()
	if err != nil {
		t.Fatalf("Recover failed: %v", err)
	}

	if len(records) != 3 {
		t.Fatalf("expected 3 recovered records, got %d", len(records))
	}
}

func TestWALMemtableFlushedFiltersRecoveredRecords(t *testing.T) {
	w := newTestWAL(t, KEY_START+64, 5)

	w.Put([]byte("k1"), []byte("v1"), 1, enums.OpTypePut)
	w.Put([]byte("k2"), []byte("v2"), 2, enums.OpTypePut)
	w.Put([]byte("k3"), []byte("v3"), 3, enums.OpTypePut)

	if err := w.MemtableFlushed(2); err != nil {
		t.Fatalf("MemtableFlushed failed: %v", err)
	}

	records, err := w.Recover()
	if err != nil {
		t.Fatalf("Recover failed: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 recovered record after flush, got %d", len(records))
	}

	if records[0].SeqId != 3 {
		t.Fatalf("expected only SeqId 3 to remain, got %d", records[0].SeqId)
	}
}
