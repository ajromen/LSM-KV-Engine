package sstable

import (
	"bytes"
	"testing"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
)

func TestIndexEntry_EncodeDecode(t *testing.T) {
	entry := IndexEntry{
		Key:    []byte("apple"),
		Offset: 12345,
	}
	buf := entry.EncodeIndexEntry()
	decoded, n, err := DecodeIndexEntry(buf)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if n != len(buf) {
		t.Fatalf("decoded bytes mismatch: %d != %d", n, len(buf))
	}
	if !bytes.Equal(decoded.Key, entry.Key) {
		t.Fatalf("key mismatch")
	}
	if decoded.Offset != entry.Offset {
		t.Fatalf("offset mismatch")
	}
}

func TestIndexBlock_EncodeDecode(t *testing.T) {
	cfg := config.NewDefaultConfig()
	ib := NewIndexBlock(cfg)
	ib.AddEntry(IndexEntry{Key: []byte("apple"), Offset: 10})
	ib.AddEntry(IndexEntry{Key: []byte("banana"), Offset: 20})
	ib.AddEntry(IndexEntry{Key: []byte("carrot"), Offset: 30})
	data := ib.Encode()
	decoded, err := DecodeIndexBlock(data)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(decoded.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(decoded.Entries))
	}
	for i := range ib.Entries {
		if !bytes.Equal(decoded.Entries[i].Key, ib.Entries[i].Key) {
			t.Fatalf("key mismatch at %d", i)
		}
		if decoded.Entries[i].Offset != ib.Entries[i].Offset {
			t.Fatalf("offset mismatch at %d", i)
		}
	}
}

func TestIndexBlock_FindBlock(t *testing.T) {
	cfg := config.NewDefaultConfig()
	ib := NewIndexBlock(cfg)
	keys := []string{"apple", "banana", "carrot", "date"}
	for i, k := range keys {
		ib.AddEntry(IndexEntry{Key: []byte(k), Offset: uint64(i)})
	}
	tests := []struct {
		key  string
		want int
	}{
		{"apple", 0},
		{"banana", 1},
		{"blueberry", 1},
		{"carrot", 2},
		{"zzz", 3},
		{"aardvark", -1},
	}
	for _, tt := range tests {
		got := ib.FindBlock([]byte(tt.key))
		if got != tt.want {
			t.Fatalf("FindBlock(%s) = %d, want %d", tt.key, got, tt.want)
		}
	}
}

func TestIndexBlock_CRCMismatch(t *testing.T) {
	cfg := config.NewDefaultConfig()
	ib := NewIndexBlock(cfg)
	ib.AddEntry(IndexEntry{Key: []byte("apple"), Offset: 1})
	data := ib.Encode()
	data[len(data)-1] ^= 0xff
	if _, err := DecodeIndexBlock(data); err == nil {
		t.Fatalf("expected CRC error")
	}
}

func TestTopLevelIndex_EncodeDecode(t *testing.T) {
	cfg := config.NewDefaultConfig()
	tli := NewTopLevelIndex(cfg)
	tli.AddEntry(TopLevelIndexEntry{
		FirstKey: []byte("apple"),
		Offset:   100,
		Size:     50,
	})
	tli.AddEntry(TopLevelIndexEntry{
		FirstKey: []byte("carrot"),
		Offset:   200,
		Size:     60,
	})
	data := tli.EncodeTopLevelIndex()
	decoded, err := DecodeTopLevelIndex(data)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(decoded.Entries) != 2 {
		t.Fatalf("expected 2 entries")
	}
	for i := range tli.Entries {
		if !bytes.Equal(decoded.Entries[i].FirstKey, tli.Entries[i].FirstKey) {
			t.Fatalf("key mismatch at %d", i)
		}
		if decoded.Entries[i].Offset != tli.Entries[i].Offset {
			t.Fatalf("offset mismatch at %d", i)
		}
		if decoded.Entries[i].Size != tli.Entries[i].Size {
			t.Fatalf("size mismatch at %d", i)
		}
	}
}

func TestTopLevelIndex_FindIndexBlock(t *testing.T) {
	cfg := config.NewDefaultConfig()
	tli := NewTopLevelIndex(cfg)
	tli.AddEntry(TopLevelIndexEntry{FirstKey: []byte("apple")})
	tli.AddEntry(TopLevelIndexEntry{FirstKey: []byte("carrot")})
	tli.AddEntry(TopLevelIndexEntry{FirstKey: []byte("eggplant")})
	tests := []struct {
		key  string
		want int
	}{
		{"apple", 0},
		{"banana", 0},
		{"carrot", 1},
		{"date", 1},
		{"zzz", 2},
		{"aardvark", -1},
	}
	for _, tt := range tests {
		got := tli.FindIndexBlock([]byte(tt.key))
		if got != tt.want {
			t.Fatalf("FindIndexBlock(%s) = %d, want %d", tt.key, got, tt.want)
		}
	}
}

func TestTwoLevelIndexBuilder_Build(t *testing.T) {
	cfg := config.NewDefaultConfig()
	builder := NewTwoLevelIndexBuilder(2, cfg)
	builder.AddDataBlock([]byte("apple"), 10)
	builder.AddDataBlock([]byte("banana"), 20)
	builder.AddDataBlock([]byte("carrot"), 30)
	builder.AddDataBlock([]byte("date"), 40)
	top, blocks := builder.Build()
	if len(blocks) != 2 {
		t.Fatalf("expected 2 index blocks, got %d", len(blocks))
	}
	if len(top.Entries) != 2 {
		t.Fatalf("expected 2 top-level entries, got %d", len(top.Entries))
	}
	if !bytes.Equal(top.Entries[0].FirstKey, []byte("apple")) {
		t.Fatalf("wrong first key in block 0")
	}
	if !bytes.Equal(top.Entries[1].FirstKey, []byte("carrot")) {
		t.Fatalf("wrong first key in block 1")
	}
}
