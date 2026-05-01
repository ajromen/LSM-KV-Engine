package wal

import (
	"bytes"
	"testing"

	"github.com/ajromen/LSM-KV-Engine/internal/enums"
)

func TestEncodeDecode(t *testing.T) {
	original := WALRecord{
		FragType: FIRST,
		RecType:  COMMIT,
		TxnID:    9,
		Record: Record{
			ExpiresAt: 88,
			SeqId:     123,
			OpType:    enums.OpTypePut,
			Key:       []byte("aa"),
			Value:     []byte("bbb"),
		},
	}

	buf := Encode(original)

	decoded, err := Decode(buf)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if original.Record.ExpiresAt != decoded.Record.ExpiresAt {
		t.Errorf("ExpiresAt mismatch: got %d, expected %d",
			decoded.Record.ExpiresAt,
			original.Record.ExpiresAt,
		)
	}

	if original.FragType != decoded.FragType {
		t.Errorf("FragType mismatch: got %d, expected %d",
			decoded.FragType,
			original.FragType,
		)
	}

	if original.Record.OpType != decoded.Record.OpType {
		t.Errorf("OpType mismatch: got %d, expected %d",
			decoded.Record.OpType,
			original.Record.OpType,
		)
	}

	if original.RecType != decoded.RecType {
		t.Errorf("RecType mismatch: got %d, expected %d",
			decoded.RecType,
			original.RecType,
		)
	}

	if original.TxnID != decoded.TxnID {
		t.Errorf("TxnID mismatch: got %d, expected %d",
			decoded.TxnID,
			original.TxnID,
		)
	}

	if original.Record.SeqId != decoded.Record.SeqId {
		t.Errorf("SeqId mismatch: got %d, expected %d",
			decoded.Record.SeqId,
			original.Record.SeqId,
		)
	}

	if uint64(len(original.Record.Key)) != decoded.KeySize {
		t.Errorf("KeySize mismatch: got %d, expected %d",
			decoded.KeySize,
			len(original.Record.Key),
		)
	}

	if uint64(len(original.Record.Value)) != decoded.ValueSize {
		t.Errorf("ValueSize mismatch: got %d, expected %d",
			decoded.ValueSize,
			len(original.Record.Value),
		)
	}

	if !bytes.Equal(original.Record.Key, decoded.Record.Key) {
		t.Errorf("Key mismatch: got %q, expected %q",
			decoded.Record.Key,
			original.Record.Key,
		)
	}

	if !bytes.Equal(original.Record.Value, decoded.Record.Value) {
		t.Errorf("Value mismatch: got %q, expected %q",
			decoded.Record.Value,
			original.Record.Value,
		)
	}
}

func TestDecodeFailsOnCRCMismatch(t *testing.T) {
	original := WALRecord{
		FragType: FULL,
		RecType:  SINGLE,
		TxnID:    0,
		Record: Record{
			ExpiresAt: 0,
			SeqId:     1,
			OpType:    enums.OpTypePut,
			Key:       []byte("key"),
			Value:     []byte("value"),
		},
	}

	buf := Encode(original)

	// Corrupt one byte after CRC so calculated CRC no longer matches stored CRC.
	buf[len(buf)-1] ^= 0xFF

	_, err := Decode(buf)
	if err == nil {
		t.Fatal("expected CRC mismatch error, got nil")
	}
}

func TestEncodeIgnoresManualKeySizeAndValueSize(t *testing.T) {
	original := WALRecord{
		FragType:  FULL,
		RecType:   SINGLE,
		TxnID:     0,
		KeySize:   999,
		ValueSize: 999,
		Record: Record{
			ExpiresAt: 0,
			SeqId:     1,
			OpType:    enums.OpTypePut,
			Key:       []byte("abc"),
			Value:     []byte("de"),
		},
	}

	buf := Encode(original)

	decoded, err := Decode(buf)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if decoded.KeySize != 3 {
		t.Errorf("expected encoded KeySize to be 3, got %d", decoded.KeySize)
	}

	if decoded.ValueSize != 2 {
		t.Errorf("expected encoded ValueSize to be 2, got %d", decoded.ValueSize)
	}
}
