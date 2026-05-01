package wal

import (
	"bytes"
	"testing"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
)

func newTestSegment(t *testing.T, blockSize int, maxBlocks int) *Segment {
	t.Helper()

	dir := t.TempDir()
	bm := block.NewBlockManager(blockSize)

	w := &WAL{
		Dir:       dir,
		BlockSize: blockSize,
		MaxBlocks: maxBlocks,
		BM:        bm,
	}

	seg, err := OpenSegment(1, w.SegmentPath(1), maxBlocks, bm)
	if err != nil {
		t.Fatalf("OpenSegment failed: %v", err)
	}

	return seg
}

func TestSegmentAppendSmallRecord(t *testing.T) {
	seg := newTestSegment(t, KEY_START+64, 3)

	rec := Record{
		ExpiresAt: 0,
		SeqId:     1,
		OpType:    enums.OpTypePut,
		Key:       []byte("hello"),
		Value:     []byte("world"),
	}

	if err := seg.Append(rec, rec.SeqId); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	if err := seg.Sync(); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	records, err := seg.ReadAllRecords()
	if err != nil {
		t.Fatalf("ReadAllRecords failed: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	got := records[0]

	if got.FragType != FULL {
		t.Fatalf("expected FULL fragment, got %v", got.FragType)
	}

	if got.RecType != SINGLE {
		t.Fatalf("expected SINGLE record type, got %v", got.RecType)
	}

	if got.Record.SeqId != rec.SeqId {
		t.Fatalf("expected SeqId %d, got %d", rec.SeqId, got.Record.SeqId)
	}

	if !bytes.Equal(got.Record.Key, rec.Key) {
		t.Fatalf("expected key %q, got %q", rec.Key, got.Record.Key)
	}

	if !bytes.Equal(got.Record.Value, rec.Value) {
		t.Fatalf("expected value %q, got %q", rec.Value, got.Record.Value)
	}
}

func TestSegmentMovesToNextBlockWhenRemainingCannotFitHeader(t *testing.T) {
	blockSize := KEY_START + 20
	seg := newTestSegment(t, blockSize, 3)

	first := Record{
		SeqId:  1,
		OpType: enums.OpTypePut,
		Key:    []byte("aaaaa"),
		Value:  []byte("bbbbbbbbbb"),
	}

	if err := seg.Append(first, first.SeqId); err != nil {
		t.Fatalf("first Append failed: %v", err)
	}

	// first record size = KEY_START + 5 + 10 = KEY_START + 15
	// remaining = 5, which is <= KEY_START, so next append should move to block 1.
	second := Record{
		SeqId:  2,
		OpType: enums.OpTypePut,
		Key:    []byte("cc"),
		Value:  []byte("dd"),
	}

	if err := seg.Append(second, second.SeqId); err != nil {
		t.Fatalf("second Append failed: %v", err)
	}

	if seg.CurrentBlockIndex != 1 {
		t.Fatalf("expected CurrentBlockIndex 1, got %d", seg.CurrentBlockIndex)
	}

	if err := seg.Sync(); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	records, err := seg.ReadAllRecords()
	if err != nil {
		t.Fatalf("ReadAllRecords failed: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	if records[0].Record.SeqId != 1 {
		t.Fatalf("expected first record SeqId 1, got %d", records[0].Record.SeqId)
	}

	if records[1].Record.SeqId != 2 {
		t.Fatalf("expected second record SeqId 2, got %d", records[1].Record.SeqId)
	}
}

func TestSegmentFragmentationFirstLast(t *testing.T) {
	seg := newTestSegment(t, KEY_START+10, 3)

	key := []byte("abcde")
	value := []byte("1234567890")

	wr := WALRecord{
		FragType: FULL,
		RecType:  SINGLE,
		TxnID:    0,
		Record: Record{
			SeqId:  10,
			OpType: enums.OpTypePut,
			Key:    key,
			Value:  value,
		},
	}

	if err := seg.AppendWALRecord(wr); err != nil {
		t.Fatalf("AppendWALRecord failed: %v", err)
	}

	if err := seg.Sync(); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	frags, err := seg.ReadAllRecords()
	if err != nil {
		t.Fatalf("ReadAllRecords failed: %v", err)
	}

	if len(frags) != 2 {
		t.Fatalf("expected 2 fragments, got %d", len(frags))
	}

	if frags[0].FragType != FIRST {
		t.Fatalf("expected first fragment FIRST, got %v", frags[0].FragType)
	}

	if frags[1].FragType != LAST {
		t.Fatalf("expected second fragment LAST, got %v", frags[1].FragType)
	}

	joined, err := JoinFragments(frags)
	if err != nil {
		t.Fatalf("JoinFragments failed: %v", err)
	}

	if len(joined) != 1 {
		t.Fatalf("expected 1 joined record, got %d", len(joined))
	}

	if !bytes.Equal(joined[0].Record.Key, key) {
		t.Fatalf("expected joined key %q, got %q", key, joined[0].Record.Key)
	}

	if !bytes.Equal(joined[0].Record.Value, value) {
		t.Fatalf("expected joined value %q, got %q", value, joined[0].Record.Value)
	}
}

func TestSegmentFragmentationFirstMiddleLast(t *testing.T) {
	seg := newTestSegment(t, KEY_START+10, 5)

	key := []byte("abcdefgh")
	value := []byte("1234567890123456789012")

	wr := WALRecord{
		FragType: FULL,
		RecType:  SINGLE,
		TxnID:    0,
		Record: Record{
			SeqId:  20,
			OpType: enums.OpTypePut,
			Key:    key,
			Value:  value,
		},
	}

	if err := seg.AppendWALRecord(wr); err != nil {
		t.Fatalf("AppendWALRecord failed: %v", err)
	}

	if err := seg.Sync(); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	frags, err := seg.ReadAllRecords()
	if err != nil {
		t.Fatalf("ReadAllRecords failed: %v", err)
	}

	if len(frags) != 3 {
		t.Fatalf("expected 3 fragments, got %d", len(frags))
	}

	if frags[0].FragType != FIRST {
		t.Fatalf("expected fragment 0 FIRST, got %v", frags[0].FragType)
	}

	if frags[1].FragType != MIDDLE {
		t.Fatalf("expected fragment 1 MIDDLE, got %v", frags[1].FragType)
	}

	if frags[2].FragType != LAST {
		t.Fatalf("expected fragment 2 LAST, got %v", frags[2].FragType)
	}

	joined, err := JoinFragments(frags)
	if err != nil {
		t.Fatalf("JoinFragments failed: %v", err)
	}

	if len(joined) != 1 {
		t.Fatalf("expected 1 joined record, got %d", len(joined))
	}

	if !bytes.Equal(joined[0].Record.Key, key) {
		t.Fatalf("expected joined key %q, got %q", key, joined[0].Record.Key)
	}

	if !bytes.Equal(joined[0].Record.Value, value) {
		t.Fatalf("expected joined value %q, got %q", value, joined[0].Record.Value)
	}
}

func TestSegmentFindBlockBySeqID(t *testing.T) {
	seg := newTestSegment(t, KEY_START+20, 3)

	first := Record{
		SeqId:  1,
		OpType: enums.OpTypePut,
		Key:    []byte("aaaaa"),
		Value:  []byte("bbbbbbbbbb"),
	}

	second := Record{
		SeqId:  2,
		OpType: enums.OpTypePut,
		Key:    []byte("cc"),
		Value:  []byte("dd"),
	}

	if err := seg.Append(first, first.SeqId); err != nil {
		t.Fatalf("first Append failed: %v", err)
	}

	if err := seg.Append(second, second.SeqId); err != nil {
		t.Fatalf("second Append failed: %v", err)
	}

	if err := seg.Sync(); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	blockID, found, err := seg.FindBlockBySeqID(2)
	if err != nil {
		t.Fatalf("FindBlockBySeqID failed: %v", err)
	}

	if !found {
		t.Fatal("expected to find SeqId 2")
	}

	if blockID != 1 {
		t.Fatalf("expected SeqId 2 in block 1, got block %d", blockID)
	}

	_, found, err = seg.FindBlockBySeqID(999)
	if err != nil {
		t.Fatalf("FindBlockBySeqID failed: %v", err)
	}

	if found {
		t.Fatal("did not expect to find SeqId 999")
	}
}
