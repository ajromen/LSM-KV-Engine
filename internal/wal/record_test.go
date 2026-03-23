package wal

import (
	"bytes"
	"testing"
)

func TestEncodeDecode(t *testing.T) {
	r := Record{
		Timestamp: 5,
		Tombstone: false,
		Key:       []byte("11"),
		Value:     []byte("1"),
	}

	buf := Encode(r)
	decoded, err := Decode(buf)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if r.Timestamp != decoded.Timestamp {
		t.Errorf("Timestamp mismatch: got %d, want %d", decoded.Timestamp, r.Timestamp)
	}
	if r.Tombstone != decoded.Tombstone {
		t.Errorf("Tombstone mismatch: got %v, want %v", decoded.Tombstone, r.Tombstone)
	}
	if !bytes.Equal(r.Key, decoded.Key) {
		t.Errorf("Key mismatch: got %v, want %v", decoded.Key, r.Key)
	}
	if !bytes.Equal(r.Value, decoded.Value) {
		t.Errorf("Key mismatch: got %v, want %v", decoded.Value, r.Value)
	}

}
