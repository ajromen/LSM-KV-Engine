package compressor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompressorBasic(t *testing.T) {
	t.Log("---- COMPRESSOR BASIC TEST ----")
	cd := NewCompressorDict()
	idx, added := cd.Add("key1")
	if !added || idx != 0 {
		t.Fatalf("expected key1 at index 0")
	}
	idx2, added := cd.Add("key2")
	if !added || idx2 != 1 {
		t.Fatalf("expected key2 at index 1")
	}
	idx3, added := cd.Add("key1")
	if added || idx3 != 0 {
		t.Fatalf("expected duplicate key1 to return existing index")
	}
	key, ok := cd.GetKey(0)
	if !ok || key != "key1" {
		t.Fatalf("expected key1 at index 0")
	}
	_, ok = cd.GetKey(10)
	if ok {
		t.Fatalf("expected invalid index to fail")
	}
}

func TestCompressorRemove(t *testing.T) {
	t.Log("---- COMPRESSOR REMOVE TEST ----")
	cd := NewCompressorDict()
	cd.Add("key1")
	cd.Add("key2")
	cd.Add("key3")
	ok := cd.Remove("key2")
	if !ok {
		t.Fatalf("expected key2 to be removed")
	}
	if cd.Size() != 2 {
		t.Fatalf("expected size 2 after remove, got %d", cd.Size())
	}
	_, ok = cd.GetIdx("key2")
	if ok {
		t.Fatalf("expected key2 to be missing")
	}
	idx, _ := cd.GetIdx("key3")
	if idx != 1 {
		t.Fatalf("expected key3 to move to index 1")
	}
}

func TestCompressorRemoveAll(t *testing.T) {
	t.Log("---- COMPRESSOR REMOVE ALL TEST ----")
	cd := NewCompressorDict()
	cd.Add("a")
	cd.Add("b")
	cd.Add("c")
	cd.RemoveAll()
	if cd.Size() != 0 {
		t.Fatalf("expected empty dict")
	}
	if len(cd.MapOfIdx()) != 0 {
		t.Fatalf("expected empty index map")
	}
}

func TestCompressorSerializeDeserialize(t *testing.T) {
	t.Log("---- COMPRESSOR SERIALIZE TEST ----")
	cd := NewCompressorDict()
	cd.Add("key1")
	cd.Add("key2")
	cd.Add("key3")
	data := cd.Serialize()
	cd2, err := Deserialize(data)
	if err != nil {
		t.Fatalf("deserialize failed: %v", err)
	}
	if cd2.Size() != 3 {
		t.Fatalf("expected size 3 after deserialize")
	}
	key, ok := cd2.GetKey(1)
	if !ok || key != "key2" {
		t.Fatalf("expected key2 at index 1")
	}
}

func TestCompressorDeserializeCorrupt(t *testing.T) {
	t.Log("---- COMPRESSOR CORRUPT DESERIALIZE TEST ----")
	data := []byte{0x05, 'a', 'b'}
	_, err := Deserialize(data)
	if err == nil {
		t.Fatalf("expected error on corrupt data")
	}
}

func TestCompressorSaveLoad(t *testing.T) {
	t.Log("---- COMPRESSOR FILE LOAD TEST ----")
	dir := t.TempDir()
	name := "dict.bin"
	cd := NewCompressorDict()
	cd.Add("key1")
	cd.Add("key2")
	err := SaveToFile(dir, name, cd.Serialize())
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}
	loaded, err := LoadFromFile(dir, name)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded.Size() != 2 {
		t.Fatalf("expected loaded size 2")
	}
	key, ok := loaded.GetKey(0)
	if !ok || key != "key1" {
		t.Fatalf("expected key1 at index 0")
	}
}

func TestCompressorAppendLast(t *testing.T) {
	t.Log("---- COMPRESSOR APPEND LAST TEST ----")
	dir := t.TempDir()
	name := "append.bin"
	path := filepath.Join(dir, name)
	cd := NewCompressorDict()
	cd.Add("key1")
	err := cd.AppendLastToFile(dir, name)
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}
	cd.Add("key2")
	err = cd.AppendLastToFile(dir, name)
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	loaded, err := Deserialize(data)
	if err != nil {
		t.Fatalf("deserialize failed: %v", err)
	}
	if loaded.Size() != 2 {
		t.Fatalf("expected 2 entries after append")
	}
	key, _ := loaded.GetKey(1)
	if key != "key2" {
		t.Fatalf("expected key2 at index 1")
	}
}

func TestCompressorAppendLastEmpty(t *testing.T) {
	t.Log("---- COMPRESSOR APPEND EMPTY TEST ----")
	cd := NewCompressorDict()
	err := cd.AppendLastToFile(t.TempDir(), "x")
	if err == nil {
		t.Fatalf("expected error when appending empty dict")
	}
}
