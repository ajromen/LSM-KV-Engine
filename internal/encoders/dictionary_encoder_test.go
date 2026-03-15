package encoders

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

func TestDictEncoderBasic(t *testing.T) {
	t.Log("---- DICT ENCODER BASIC TEST ----")
	enc := NewDictEncoder(nil)
	keys := [][]byte{
		[]byte("apple"),
		[]byte("banana"),
		[]byte("cherry"),
	}
	for i, k := range keys {
		idx, added := enc.AddToDict(k)
		if !added {
			t.Fatalf("expected key %s to be added", k)
		}
		if idx != uint64(i) {
			t.Fatalf("expected index %d, got %d", i, idx)
		}
	}

	// Check duplicates
	idx, added := enc.AddToDict([]byte("apple"))
	if added {
		t.Fatalf("expected duplicate key not to be added")
	}
	if idx != 0 {
		t.Fatalf("expected index 0 for duplicate, got %d", idx)
	}

	// Encode/decode
	for _, k := range keys {
		encBytes := enc.Encode(k)
		decKey := enc.Decode(encBytes)
		if !bytes.Equal([]byte(decKey), k) {
			t.Fatalf("expected decoded key %s, got %s", k, decKey)
		}
	}
}

func TestDictEncoderBitWidth(t *testing.T) {
	t.Log("---- DICT ENCODER BITWIDTH TEST ----")
	enc := NewDictEncoder(nil)
	for i := 0; i < 16; i++ {
		enc.AddToDict([]byte(fmt.Sprintf("%d", i)))
	}
	bw := enc.BitWidth()
	if bw != 4 {
		t.Fatalf("expected bitWidth 4 for 16 keys, got %d", bw)
	}

	enc.AddToDict([]byte("z"))
	bw = enc.BitWidth()
	if bw != 5 {
		t.Fatalf("expected bitWidth 5 after adding 17th key, got %d", bw)
	}
}

func TestDictEncoderBuildDictionary(t *testing.T) {
	t.Log("---- DICT ENCODER BUILD DICTIONARY TEST ----")
	enc := NewDictEncoder(nil)
	keys := [][]byte{
		[]byte("dog"),
		[]byte("cat"),
		[]byte("bird"),
	}
	enc.BuildDictionary(keys)
	if enc.Size() != 3 {
		t.Fatalf("expected size 3, got %d", enc.Size())
	}
	expectedKeys := []string{"bird", "cat", "dog"} // sorted
	for i, k := range expectedKeys {
		if enc.Key(uint64(i)) != k {
			t.Fatalf("expected key at index %d to be %s, got %s", i, k, enc.Key(uint64(i)))
		}
	}
}

func TestDictEncoderSaveLoad(t *testing.T) {
	t.Log("---- DICT ENCODER SAVE/LOAD TEST ----")
	enc := NewDictEncoder(nil)
	keys := [][]byte{
		[]byte("red"),
		[]byte("green"),
		[]byte("blue"),
	}
	for _, k := range keys {
		enc.AddToDict(k)
	}
	data := enc.SaveDict()

	enc2 := NewDictEncoder(data)
	if enc2.Size() != len(keys) {
		t.Fatalf("expected size %d after load, got %d", len(keys), enc2.Size())
	}
	for _, k := range keys {
		idx, ok := enc2.Index(string(k))
		if !ok {
			t.Fatalf("key %s missing after load", k)
		}
		if enc2.Key(idx) != string(k) {
			t.Fatalf("key mismatch after load: expected %s, got %s", k, enc2.Key(idx))
		}
	}
}

func TestDictEncoderFileIO(t *testing.T) {
	t.Log("---- DICT ENCODER FILE IO TEST ----")
	tmpFile, err := os.CreateTemp("", "dictenc_test")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	enc := NewDictEncoder(nil)
	keys := [][]byte{
		[]byte("one"),
		[]byte("two"),
		[]byte("three"),
	}
	for _, k := range keys {
		enc.AddToDict(k)
	}

	if err := enc.WriteToFile(tmpFile, 0); err != nil {
		t.Fatalf("failed to write dictionary to file: %v", err)
	}

	size := len(enc.SaveDict())
	enc2 := NewDictEncoder(nil)
	if err := enc2.ReadDictFromFile(tmpFile, 0, size); err != nil {
		t.Fatalf("failed to read dictionary from file: %v", err)
	}

	if enc2.Size() != len(keys) {
		t.Fatalf("expected size %d, got %d", len(keys), enc2.Size())
	}
}
