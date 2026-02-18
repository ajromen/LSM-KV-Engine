package encoders

import (
	"testing"
)

func TestDeltaEncoderIntBasic(t *testing.T) {
	t.Log("---- DELTA ENCODER INT BASIC TEST ----")
	enc := NewDeltaEncoderInt(3)
	buf := make([]byte, 0)
	offset := uint32(0)
	keys := []int{10, 12, 15, 20, 25}
	for _, k := range keys {
		buf = enc.Encode(k, offset, buf)
		offset++
	}
	enc.Reset()
	pos := 0
	for i, expected := range keys {
		key, err := enc.Decode(buf, &pos)
		if err != nil {
			t.Fatalf("decode failed at index %d: %v", i, err)
		}
		if key != expected {
			t.Fatalf("expected key %d, got %d", expected, key)
		}
	}
}

func TestDeltaEncoderIntRestartInterval(t *testing.T) {
	t.Log("---- DELTA ENCODER INT RESTART TEST ----")
	enc := NewDeltaEncoderInt(2)
	buf := make([]byte, 0)
	offset := uint32(0)
	keys := []int{5, 6, 20, 22}
	for _, k := range keys {
		buf = enc.Encode(k, offset, buf)
		offset++
	}
	if len(enc.restartArray) != 2 {
		t.Fatalf("expected 2 restart, got %d", len(enc.restartArray))
	}
	enc.Reset()
	pos := 0
	for i, expected := range keys {
		key, err := enc.Decode(buf, &pos)
		if err != nil {
			t.Fatalf("decode failed at index %d: %v", i, err)
		}
		if key != expected {
			t.Fatalf("expected key %d, got %d", expected, key)
		}
	}
}

func TestDeltaEncoderIntNegativeDelta(t *testing.T) {
	t.Log("---- DELTA ENCODER INT NEGATIVE DELTA TEST ----")
	enc := NewDeltaEncoderInt(10)
	buf := make([]byte, 0)
	keys := []int{100, 90, 80, 85, 70}
	for i, k := range keys {
		buf = enc.Encode(k, uint32(i), buf)
	}
	enc.Reset()
	pos := 0
	for i, expected := range keys {
		key, err := enc.Decode(buf, &pos)
		if err != nil {
			t.Fatalf("decode failed at index %d: %v", i, err)
		}
		if key != expected {
			t.Fatalf("expected key %d, got %d", expected, key)
		}
	}
}

func TestDeltaEncoderIntWriteRestartArray(t *testing.T) {
	t.Log("---- DELTA ENCODER INT RESTART ARRAY TEST ----")
	enc := NewDeltaEncoderInt(1)
	buf := make([]byte, 0)
	enc.Encode(1, 10, buf)
	enc.Encode(2, 20, buf)
	enc.Encode(3, 30, buf)
	restartBuf := make([]byte, 0)
	restartBuf = enc.WriteRestartArray(restartBuf)
	if len(enc.restartArray) != 3 {
		t.Fatalf("expected 3 restarts, got %d", len(enc.restartArray))
	}
	if len(restartBuf) == 0 {
		t.Fatalf("expected restart buffer to be written")
	}
}
