package sstable

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"testing"

	"github.com/ajromen/LSM-KV-Engine/internal/shared"
)

func TestIndexEntryEncodeDecode(t *testing.T) {
	entry := shared.IndexEntry{
		Key:        []byte("key1"),
		BlockIndex: 42,
	}

	buf := make([]byte, entry.EncodedSize())
	n := entry.EncodeTo(buf)

	decoded, read, err := shared.DecodeIndexEntry(buf)
	if err != nil {
		t.Fatalf("failed to decode index entry: %v", err)
	}
	if read != n {
		t.Fatalf("expected to consume %d bytes, got %d", n, read)
	}
	if !bytes.Equal(decoded.Key, entry.Key) {
		t.Fatalf("expected key %s, got %s", entry.Key, decoded.Key)
	}
	if decoded.BlockIndex != entry.BlockIndex {
		t.Fatalf("expected block index %d, got %d", entry.BlockIndex, decoded.BlockIndex)
	}
}

func TestIndexBlockEncodeDecode(t *testing.T) {
	block := NewIndexBlock()
	block.AddEntry(shared.IndexEntry{Key: []byte("a"), BlockIndex: 10})
	block.AddEntry(shared.IndexEntry{Key: []byte("b"), BlockIndex: 20})
	block.AddEntry(shared.IndexEntry{Key: []byte("c"), BlockIndex: 30})

	encoded := block.EncodeIndexBlock(128)

	dataWithoutCRC := encoded[:len(encoded)-4]
	expectedCRC := binary.LittleEndian.Uint32(encoded[len(encoded)-4:])
	actualCRC := crc32.ChecksumIEEE(dataWithoutCRC)

	if expectedCRC != actualCRC {
		t.Fatalf("crc mismatch: expected %d, got %d", expectedCRC, actualCRC)
	}

	decoded, err := DecodeIndexBlock(encoded)
	if err != nil {
		t.Fatalf("failed to decode index block: %v", err)
	}

	if len(decoded.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(decoded.Entries))
	}

	if string(decoded.Entries[0].Key) != "a" {
		t.Fatalf("expected key a, got %s", decoded.Entries[0].Key)
	}
	if decoded.Entries[2].BlockIndex != 30 {
		t.Fatalf("expected block index 30, got %d", decoded.Entries[2].BlockIndex)
	}
}

func TestIndexBlockFindBlock(t *testing.T) {
	block := NewIndexBlock()
	block.AddEntry(shared.IndexEntry{Key: []byte("a"), BlockIndex: 0})
	block.AddEntry(shared.IndexEntry{Key: []byte("d"), BlockIndex: 100})
	block.AddEntry(shared.IndexEntry{Key: []byte("g"), BlockIndex: 200})

	tests := []struct {
		key      string
		expected int
	}{
		{"a", 0},
		{"b", 0},
		{"e", 1},
		{"z", 2},
		{"0", 0}, // ⚠️ changed: no longer -1
	}

	for _, tt := range tests {
		idx := block.FindBlock([]byte(tt.key))
		if idx != tt.expected {
			t.Fatalf("key %s: expected %d, got %d", tt.key, tt.expected, idx)
		}
	}
}

func TestIndexSegmentAddEntryToBlock(t *testing.T) {
	seg := NewIndexSegment(160)

	seg.AddEntryToBlock(shared.IndexEntry{Key: []byte("a"), BlockIndex: 1}, 2)
	seg.AddEntryToBlock(shared.IndexEntry{Key: []byte("b"), BlockIndex: 2}, 2)
	seg.AddEntryToBlock(shared.IndexEntry{Key: []byte("c"), BlockIndex: 3}, 2)

	if len(seg.Blocks) != 2 {
		t.Fatalf("expected 2 index blocks, got %d", len(seg.Blocks))
	}

	if len(seg.Blocks[0].Entries) != 2 {
		t.Fatalf("expected first block to have 2 entries, got %d", len(seg.Blocks[0].Entries))
	}

	if len(seg.Blocks[1].Entries) != 1 {
		t.Fatalf("expected second block to have 1 entry, got %d", len(seg.Blocks[1].Entries))
	}
}

func TestIndexSegmentGetBlockSizes(t *testing.T) {
	seg := NewIndexSegment(160)

	block1 := NewIndexBlock()
	block1.AddEntry(shared.IndexEntry{Key: []byte("a"), BlockIndex: 1})

	block2 := NewIndexBlock()
	block2.AddEntry(shared.IndexEntry{Key: []byte("b"), BlockIndex: 2})
	block2.AddEntry(shared.IndexEntry{Key: []byte("c"), BlockIndex: 3})

	seg.AddBlock(block1)
	seg.AddBlock(block2)

	sizes := seg.GetBlockSizes()

	if len(sizes) != 2 {
		t.Fatalf("expected 2 block sizes, got %d", len(sizes))
	}

	if sizes[0] != block1.RealSize {
		t.Fatalf("expected block1 size %d, got %d", block1.RealSize, sizes[0])
	}
	if sizes[1] != block2.RealSize {
		t.Fatalf("expected block2 size %d, got %d", block2.RealSize, sizes[1])
	}
}
