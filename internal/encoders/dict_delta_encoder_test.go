package encoders

import (
	"bytes"
	"testing"
)

func TestDictDeltaEncoderBasic(t *testing.T) {
	t.Log("---- DICT DELTA ENCODER BASIC TEST ----")

	enc := NewDictDeltaEncoder(3)
	buf := make([]byte, 0)

	keys := []string{"key1", "key2", "key3", "key4"}

	for i, k := range keys {
		buf = enc.Encode([]byte(k), uint32(i), buf)
	}

	enc.Reset()

	pos := 0
	for i, expected := range keys {
		key, err := enc.Decode(buf, &pos)
		if err != nil {
			t.Fatalf("decode failed at index %d: %v", i, err)
		}
		if bytes.Compare(key, []byte(expected)) != 0 {
			t.Fatalf("expected key %s, got %s", expected, key)
		}
	}
}

func TestDictDeltaEncoderRestartInterval(t *testing.T) {
	t.Log("---- DICT DELTA ENCODER RESTART TEST ----")

	enc := NewDictDeltaEncoder(2)
	buf := make([]byte, 0)

	keys := []string{"a", "b", "c", "d"}

	for i, k := range keys {
		buf = enc.Encode([]byte(k), uint32(i), buf)
	}

	if len(enc.deltaEncoder.restartArray) != 2 {
		t.Fatalf("expected 2 restarts, got %d", len(enc.deltaEncoder.restartArray))
	}

	enc.Reset()

	pos := 0
	for i, expected := range keys {
		key, err := enc.Decode(buf, &pos)
		if err != nil {
			t.Fatalf("decode failed at index %d: %v", i, err)
		}
		if bytes.Compare(key, []byte(expected)) != 0 {
			t.Fatalf("expected key %s, got %s", expected, key)
		}
	}
}

func TestDictDeltaEncoderDuplicateKeys(t *testing.T) {
	t.Log("---- DICT DELTA ENCODER DUPLICATE KEY TEST ----")

	enc := NewDictDeltaEncoder(5)
	buf := make([]byte, 0)

	keys := []string{"x", "y", "x", "y", "x"}

	for i, k := range keys {
		buf = enc.Encode([]byte(k), uint32(i), buf)
	}

	if enc.DictionarySize() != 2 {
		t.Fatalf("expected dictionary size 2, got %d", enc.DictionarySize())
	}

	enc.Reset()

	pos := 0
	for i, expected := range keys {
		key, err := enc.Decode(buf, &pos)
		if err != nil {
			t.Fatalf("decode failed at index %d: %v", i, err)
		}
		if bytes.Compare(key, []byte(expected)) != 0 {
			t.Fatalf("expected key %s, got %s", expected, key)
		}
	}
}

func TestDictDeltaEncoderSerializeDictionary(t *testing.T) {
	t.Log("---- DICT DELTA ENCODER SERIALIZE DICTIONARY TEST ----")

	enc := NewDictDeltaEncoder(3)
	buf := make([]byte, 0)

	keys := []string{"apple", "banana", "cherry"}

	for i, k := range keys {
		buf = enc.Encode([]byte(k), uint32(i), buf)
	}

	dictBytes := enc.SerializeDictionary()

	newEnc := NewDictDeltaEncoder(3)
	err := newEnc.LoadDictionary(dictBytes)
	if err != nil {
		t.Fatalf("failed to load dictionary: %v", err)
	}

	if newEnc.DictionarySize() != len(keys) {
		t.Fatalf("expected dictionary size %d, got %d", len(keys), newEnc.DictionarySize())
	}
}
