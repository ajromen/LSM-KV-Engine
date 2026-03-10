package encoders

import (
	"bytes"
	"testing"
)

func TestDeltaEncoderBytesBasic(t *testing.T) {
	t.Log("---- DELTA ENCODER BYTES BASIC TEST ----")
	enc := NewDeltaEncoderBytes(3)
	buf := make([]byte, 0)
	offset := uint32(0)
	keys := [][]byte{
		[]byte("apple"),
		[]byte("apples"),
		[]byte("application"),
		[]byte("banana"),
		[]byte("band"),
	}
	for _, k := range keys {
		buf = enc.Encode(k, offset, buf)
		offset += uint32(len(k))
	}
	enc.Reset()
	pos := 0
	for i, expected := range keys {
		key, err := enc.Decode(buf, &pos)
		if err != nil {
			t.Fatalf("decode failed at index %d: %v", i, err)
		}
		if !bytes.Equal(key, expected) {
			t.Fatalf("expected key %s, got %s", expected, key)
		}
	}
}

func TestDeltaEncoderBytesRestartInterval(t *testing.T) {
	t.Log("---- DELTA ENCODER BYTES RESTART TEST ----")
	enc := NewDeltaEncoderBytes(2)
	buf := make([]byte, 0)

	keys := [][]byte{
		[]byte("car"),
		[]byte("cart"),
		[]byte("dog"),
		[]byte("dove"),
	}
	offset := uint32(0)
	for _, k := range keys {
		buf = enc.Encode(k, offset, buf)
		offset++
	}
	if len(enc.RestartArray()) != 2 {
		t.Fatalf("expected 2 restart, got %d", len(enc.RestartArray()))
	}
	enc.Reset()
	pos := 0
	for i, expected := range keys {
		key, err := enc.Decode(buf, &pos)
		if err != nil {
			t.Fatalf("decode failed at index %d: %v", i, err)
		}
		if !bytes.Equal(key, expected) {
			t.Fatalf("expected key %s, got %s", expected, key)
		}
	}
}

func TestDeltaEncoderBytesWriteRestartArray(t *testing.T) {
	t.Log("---- DELTA ENCODER BYTES RESTART ARRAY TEST ----")
	enc := NewDeltaEncoderBytes(1)
	buf := make([]byte, 0)
	buf = enc.Encode([]byte("a"), 10, buf)
	buf = enc.Encode([]byte("b"), 20, buf)
	buf = enc.Encode([]byte("c"), 30, buf)
	restartBuf := make([]byte, 0)
	restartBuf = enc.WriteRestartArray(restartBuf)
	if len(enc.RestartArray()) != 3 {
		t.Fatalf("expected 3 restarts, got %d", len(enc.RestartArray()))
	}
	if len(restartBuf) == 0 {
		t.Fatalf("expected restart buffer to be written")
	}
}
