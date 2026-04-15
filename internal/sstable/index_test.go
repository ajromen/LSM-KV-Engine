package sstable

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"testing"
)

func TestIndexEntryEncodeDecode(t *testing.T) {
	t.Log("---- INDEX ENTRY ENCODE/DECODE TEST ----")

	entry := IndexEntry{
		Key:        []byte("key1"),
		BlockIndex: 42,
	}

	encoded := entry.EncodeIndexEntry()
	decoded, n, err := DecodeIndexEntry(encoded)
	if err != nil {
		t.Fatalf("failed to decode index entry: %v", err)
	}
	if n != len(encoded) {
		t.Fatalf("expected to consume %d bytes, got %d", len(encoded), n)
	}
	if !bytes.Equal(decoded.Key, entry.Key) {
		t.Fatalf("expected key %s, got %s", entry.Key, decoded.Key)
	}
	if decoded.BlockIndex != entry.BlockIndex {
		t.Fatalf("expected block index %d, got %d", entry.BlockIndex, decoded.BlockIndex)
	}
}

func TestIndexBlockEncodeDecode(t *testing.T) {
	t.Log("---- INDEX BLOCK ENCODE/DECODE TEST ----")

	block := NewIndexBlock()
	block.AddEntry(IndexEntry{Key: []byte("a"), BlockIndex: 10})
	block.AddEntry(IndexEntry{Key: []byte("b"), BlockIndex: 20})
	block.AddEntry(IndexEntry{Key: []byte("c"), BlockIndex: 30})

	encoded := block.EncodeIndexBlock(50)

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
	t.Log("---- INDEX BLOCK FIND BLOCK TEST ----")

	block := NewIndexBlock()
	block.AddEntry(IndexEntry{Key: []byte("a"), BlockIndex: 0})
	block.AddEntry(IndexEntry{Key: []byte("d"), BlockIndex: 100})
	block.AddEntry(IndexEntry{Key: []byte("g"), BlockIndex: 200})

	idx := block.FindBlock([]byte("a"))
	if idx != 0 {
		t.Fatalf("expected index 0 for key a, got %d", idx)
	}

	idx = block.FindBlock([]byte("b"))
	if idx != 0 {
		t.Fatalf("expected index 0 for key b, got %d", idx)
	}

	idx = block.FindBlock([]byte("e"))
	if idx != 1 {
		t.Fatalf("expected index 1 for key e, got %d", idx)
	}

	idx = block.FindBlock([]byte("z"))
	if idx != 2 {
		t.Fatalf("expected index 2 for key z, got %d", idx)
	}

	idx = block.FindBlock([]byte("0"))
	if idx != -1 {
		t.Fatalf("expected -1 for key smaller than first, got %d", idx)
	}
}

func TestIndexSegmentAddEntryToBlock(t *testing.T) {
	t.Log("---- INDEX SEGMENT ADD ENTRY TO BLOCK TEST ----")

	seg := NewIndexSegment(160)

	seg.AddEntryToBlock(IndexEntry{Key: []byte("a"), BlockIndex: 1}, 2)
	seg.AddEntryToBlock(IndexEntry{Key: []byte("b"), BlockIndex: 2}, 2)
	seg.AddEntryToBlock(IndexEntry{Key: []byte("c"), BlockIndex: 3}, 2)

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
	t.Log("---- INDEX SEGMENT GET BLOCK SIZES TEST ----")

	seg := NewIndexSegment(160)

	block1 := NewIndexBlock()
	block1.AddEntry(IndexEntry{Key: []byte("a"), BlockIndex: 1})

	block2 := NewIndexBlock()
	block2.AddEntry(IndexEntry{Key: []byte("b"), BlockIndex: 2})
	block2.AddEntry(IndexEntry{Key: []byte("c"), BlockIndex: 3})

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
