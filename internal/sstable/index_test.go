package sstable

import (
	"bytes"
	"testing"
)

// --- MOCK DataBlockBuilder ---
type MockDataBlockBuilder struct {
	keys [][]byte
}

func (m *MockDataBlockBuilder) FirstKey() []byte {
	if len(m.keys) == 0 {
		return nil
	}
	return m.keys[0]
}

// ----------------------------

func TestIndexBlockAddEntry(t *testing.T) {
	t.Log("---- INDEXBLOCK ADD ENTRY TEST ----")
	ib := NewIndexBlock()
	entry := IndexEntry{
		Key:    []byte("apple"),
		Offset: 42,
	}
	ib.AddEntry(entry)

	if len(ib.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(ib.Entries))
	}
	if !bytes.Equal(ib.Entries[0].Key, []byte("apple")) {
		t.Fatalf("expected key apple, got %s", ib.Entries[0].Key)
	}
	if ib.Entries[0].Offset != 42 {
		t.Fatalf("expected offset 42, got %d", ib.Entries[0].Offset)
	}
}

func TestIndexBlockEncodeDecode(t *testing.T) {
	t.Log("---- INDEXBLOCK ENCODE/DECODE TEST ----")
	ib := NewIndexBlock()
	ib.AddEntry(IndexEntry{Key: []byte("apple"), Offset: 1})
	ib.AddEntry(IndexEntry{Key: []byte("banana"), Offset: 2})

	data := ib.EncodeIndexBlock()
	decoded, err := DecodeIndexBlock(data)
	if err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if len(decoded.Entries) != 2 {
		t.Fatalf("expected 2 entries after decode, got %d", len(decoded.Entries))
	}
	if !bytes.Equal(decoded.Entries[1].Key, []byte("banana")) {
		t.Fatalf("expected second key banana, got %s", decoded.Entries[1].Key)
	}
	if decoded.Entries[1].Offset != 2 {
		t.Fatalf("expected second offset 2, got %d", decoded.Entries[1].Offset)
	}
}

func TestIndexBlockFindBlock(t *testing.T) {
	t.Log("---- INDEXBLOCK FIND BLOCK TEST ----")
	ib := NewIndexBlock()
	keys := [][]byte{[]byte("apple"), []byte("banana"), []byte("cherry")}
	for i, k := range keys {
		ib.AddEntry(IndexEntry{Key: k, Offset: uint64(i)})
	}

	tests := []struct {
		key      []byte
		expected int
	}{
		{[]byte("aardvark"), -1},
		{[]byte("apple"), 0},
		{[]byte("banana"), 1},
		{[]byte("blueberry"), 1},
		{[]byte("cherry"), 2},
		{[]byte("date"), 2},
	}

	for _, tt := range tests {
		idx := ib.FindBlock(tt.key)
		if idx != tt.expected {
			t.Fatalf("FindBlock(%s) = %d; want %d", tt.key, idx, tt.expected)
		}
	}
}
