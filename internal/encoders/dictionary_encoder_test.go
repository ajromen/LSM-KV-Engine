package encoders

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCompressorBasic(t *testing.T) {
	t.Log("---- COMPRESSOR BASIC TEST ----")
	encoder := NewDictionaryEncoder()
	idx, AddToDicted := encoder.AddToDict([]byte("key1"))
	if !AddToDicted || idx != 0 {
		t.Fatalf("expected key1 at index 0")
	}
	idx2, AddToDicted := encoder.AddToDict([]byte("key2"))
	if !AddToDicted || idx2 != 1 {
		t.Fatalf("expected key2 at index 1")
	}
	idx3, AddToDicted := encoder.AddToDict([]byte("key1"))
	if AddToDicted || idx3 != 0 {
		t.Fatalf("expected duplicate key1 to return existing index")
	}
	key, error := encoder.GetKey(0)
	if error != nil || bytes.Compare(key, []byte("key1")) != 0 {
		t.Fatalf("expected key1 at index 0")
	}
	_, error = encoder.GetKey(10)
	if error == nil {
		t.Fatalf("expected invalid index to fail")
	}
}

func TestCompressorRemoveFromDict(t *testing.T) {
	t.Log("---- COMPRESSOR RemoveFromDict TEST ----")
	encoder := NewDictionaryEncoder()
	encoder.AddToDict([]byte("key1"))
	encoder.AddToDict([]byte("key2"))
	encoder.AddToDict([]byte("key3"))
	ok := encoder.RemoveFromDict([]byte("key2"))
	if !ok {
		t.Fatalf("expected key2 to be RemoveFromDictd")
	}
	if encoder.Size() != 2 {
		t.Fatalf("expected size 2 after RemoveFromDict, got %d", encoder.Size())
	}
	_, ok = encoder.GetIdx([]byte("key2"))
	if ok {
		t.Fatalf("expected key2 to be missing")
	}
	idx, _ := encoder.GetIdx([]byte("key3"))
	if idx != 1 {
		t.Fatalf("expected key3 to move to index 1")
	}
}

func TestCompressorRemoveFromDictAll(t *testing.T) {
	t.Log("---- COMPRESSOR RemoveFromDict ALL TEST ----")
	encoder := NewDictionaryEncoder()
	encoder.AddToDict([]byte("a"))
	encoder.AddToDict([]byte("b"))
	encoder.AddToDict([]byte("c"))
	encoder.RemoveAll()
	if encoder.Size() != 0 {
		t.Fatalf("expected empty dict")
	}
	if len(encoder.GetMapOfIdx()) != 0 {
		t.Fatalf("expected empty index map")
	}
}

func TestCompressorSerializeDeserialize(t *testing.T) {
	t.Log("---- COMPRESSOR SERIALIZE TEST ----")
	encoder := NewDictionaryEncoder()
	encoder.AddToDict([]byte("key1"))
	encoder.AddToDict([]byte("key2"))
	encoder.AddToDict([]byte("key3"))
	data := encoder.Serialize()
	encoder2, err := Deserialize(data)
	if err != nil {
		t.Fatalf("deserialize failed: %v", err)
	}
	if encoder2.Size() != 3 {
		t.Fatalf("expected size 3 after deserialize")
	}
	key, error := encoder2.GetKey(1)
	if error != nil || bytes.Compare(key, []byte("key2")) != 0 {
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
	encoder := NewDictionaryEncoder()
	encoder.AddToDict([]byte("key1"))
	encoder.AddToDict([]byte("key2"))
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, encoder.Serialize(), 0644)
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
	key, error := loaded.GetKey(0)
	if error != nil || bytes.Compare(key, []byte("key1")) != 0 {
		t.Fatalf("expected key1 at index 0")
	}
}

func TestCompressorAppendLast(t *testing.T) {
	t.Log("---- COMPRESSOR APPEND LAST TEST ----")
	dir := t.TempDir()
	name := "append.bin"
	path := filepath.Join(dir, name)
	encoder := NewDictionaryEncoder()
	encoder.AddToDict([]byte("key1"))
	err := encoder.AppendLastToFile(dir, name)
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}
	encoder.AddToDict([]byte("key2"))
	err = encoder.AppendLastToFile(dir, name)
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
	if bytes.Compare(key, []byte("key2")) != 0 {
		t.Fatalf("expected key2 at index 1")
	}
}

func TestCompressorAppendLastEmpty(t *testing.T) {
	t.Log("---- COMPRESSOR APPEND EMPTY TEST ----")
	encoder := NewDictionaryEncoder()
	err := encoder.AppendLastToFile(t.TempDir(), "x")
	if err == nil {
		t.Fatalf("expected error when appending empty dict")
	}
}
