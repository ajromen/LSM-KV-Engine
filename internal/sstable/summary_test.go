package sstable

import (
	"bytes"
	"os"
	"testing"

	"github.com/ajromen/LSM-KV-Engine/internal/shared"
)

func makeTestIndexBlocks() ([]*IndexBlock, []uint64) {
	blocks := []*IndexBlock{}
	offsets := []uint64{}
	for i := 0; i < 5; i++ {
		block := &IndexBlock{
			Entries: []shared.IndexEntry{
				{Key: []byte{byte('a' + i)}, BlockIndex: uint32(i * 10)},
			},
		}
		blocks = append(blocks, block)
		offsets = append(offsets, uint64(i*100))
	}
	return blocks, offsets
}

func TestSummarySegmentEncodeDecode(t *testing.T) {
	t.Log("---- SUMMARY SEGMENT ENCODE/DECODE TEST ----")

	blocks, offsets := makeTestIndexBlocks()
	summary := BuildSummaryFromIndex(offsets, blocks, 2)

	data := summary.Encode()
	decoded, err := DecodeSummarySegment(data)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if !bytes.Equal(decoded.MinKey, summary.MinKey) {
		t.Fatalf("MinKey mismatch")
	}
	if !bytes.Equal(decoded.MaxKey, summary.MaxKey) {
		t.Fatalf("MaxKey mismatch")
	}
	if decoded.SamplingDegree != summary.SamplingDegree {
		t.Fatalf("SamplingDegree mismatch")
	}
	if decoded.TotalIndexBlocks != summary.TotalIndexBlocks {
		t.Fatalf("TotalIndexBlocks mismatch")
	}
	if len(decoded.Entries) != len(summary.Entries) {
		t.Fatalf("Entries count mismatch")
	}
	for i := range decoded.Entries {
		if !bytes.Equal(decoded.Entries[i].Key, summary.Entries[i].Key) {
			t.Fatalf("entry %d key mismatch", i)
		}
		if decoded.Entries[i].IndexBlockOffset != summary.Entries[i].IndexBlockOffset {
			t.Fatalf("entry %d offset mismatch", i)
		}
	}
}

func TestSummarySegmentFindIndexBlockNumber(t *testing.T) {
	t.Log("---- SUMMARY SEGMENT FIND INDEX BLOCK NUMBER ----")

	blocks, offsets := makeTestIndexBlocks()
	summary := BuildSummaryFromIndex(offsets, blocks, 2)

	tests := []struct {
		key      []byte
		expected int
	}{
		{[]byte("a"), 0},
		{[]byte("b"), 0},
		{[]byte("c"), 2},
		{[]byte("d"), 2},
		{[]byte("e"), 4},
		{[]byte("f"), 4},
		{[]byte("z"), 4},
	}

	for _, tt := range tests {
		got := summary.FindIndexBlockNumber(tt.key)
		if got != tt.expected {
			t.Fatalf("key %s: expected %d, got %d", tt.key, tt.expected, got)
		}
	}
}

func TestSummarySegmentFindBlockRange(t *testing.T) {
	t.Log("---- SUMMARY SEGMENT FIND BLOCK RANGE ----")

	blocks, offsets := makeTestIndexBlocks()
	summary := BuildSummaryFromIndex(offsets, blocks, 2)

	start, end := summary.FindBlockRange([]byte("a"), []byte("d"))
	if start != 0 || end != 3 {
		t.Fatalf("unexpected block range: start=%d, end=%d", start, end)
	}

	start, end = summary.FindBlockRange([]byte("c"), []byte("z"))
	if start != 2 || end != 4 {
		t.Fatalf("unexpected block range: start=%d, end=%d", start, end)
	}
}

func TestSummarySegmentWriteReadFile(t *testing.T) {
	t.Log("---- SUMMARY SEGMENT WRITE/READ FILE ----")

	blocks, offsets := makeTestIndexBlocks()
	summary := BuildSummaryFromIndex(offsets, blocks, 2)

	tmpFile, err := os.CreateTemp("", "summaryseg_test")
	if err != nil {
		t.Fatalf("temp file creation failed: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	data := summary.Encode()
	n, err := tmpFile.Write(data)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if n == 0 {
		t.Fatalf("no bytes written")
	}

	if _, err := tmpFile.Seek(0, 0); err != nil {
		t.Fatalf("seek failed: %v", err)
	}
	readData := make([]byte, n)
	if _, err := tmpFile.Read(readData); err != nil {
		t.Fatalf("read failed: %v", err)
	}
	readSummary, err := DecodeSummarySegment(readData)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if !bytes.Equal(readSummary.MinKey, summary.MinKey) {
		t.Fatalf("MinKey mismatch")
	}
	if !bytes.Equal(readSummary.MaxKey, summary.MaxKey) {
		t.Fatalf("MaxKey mismatch")
	}
	if readSummary.TotalIndexBlocks != summary.TotalIndexBlocks {
		t.Fatalf("TotalIndexBlocks mismatch")
	}
	if len(readSummary.Entries) != len(summary.Entries) {
		t.Fatalf("Entries count mismatch")
	}
}
