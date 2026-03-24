package wal

import (
	"bytes"
	"testing"
)

func TestEncodeDecode(t *testing.T) {
	r := WALRecord{
		FragType:  FIRST,
		RecType:   COMMIT,
		TxnID:     9,
		KeySize:   2,
		ValueSize: 3,
		Record: Record{
			Timestamp: 88,
			Tombstone: false,
			Key:       []byte("aa"),
			Value:     []byte("bbb"),
		},
	}

	buf := Encode(r)
	decoded, err := Decode(buf)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if r.Record.Timestamp != decoded.Record.Timestamp {
		t.Errorf("Timestamp mismatch: got %d, expected %d", decoded.Record.Timestamp, r.Record.Timestamp)
	}
	if r.FragType != decoded.FragType {
		t.Errorf("FragType mismatch: got %d, expected %d", decoded.FragType, r.FragType)
	}
	if r.Record.Tombstone != decoded.Record.Tombstone {
		t.Errorf("Tombstone mismatch: got %v, expected %v", decoded.Record.Tombstone, r.Record.Tombstone)
	}
	if r.RecType != decoded.RecType {
		t.Errorf("RecType mismatch: got %d, expected %d", decoded.RecType, r.RecType)
	}
	if r.TxnID != decoded.TxnID {
		t.Errorf("Transaction ID mismatch: got %d, expected %d", decoded.TxnID, r.TxnID)
	}
	if r.KeySize != decoded.KeySize {
		t.Errorf("KeySize mismatch: got %d, expected %d", decoded.KeySize, r.KeySize)
	}
	if r.ValueSize != decoded.ValueSize {
		t.Errorf("ValueSize mismatch: got %d, expected %d", decoded.ValueSize, r.ValueSize)
	}

	if !bytes.Equal(r.Record.Key, decoded.Record.Key) {
		t.Errorf("Key mismatch: got %d, expected %d", decoded.Record.Key, r.Record.Key)
	}
	if !bytes.Equal(r.Record.Value, decoded.Record.Value) {
		t.Errorf("Value mismatch: got %d, expected %d", decoded.Record.Value, r.Record.Value)
	}

}
