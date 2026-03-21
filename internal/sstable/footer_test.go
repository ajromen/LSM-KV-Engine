package sstable

import (
	"encoding/binary"
	"hash/crc32"
	"testing"

	"github.com/ajromen/LSM-KV-Engine/internal/utils"
)

func TestFooterEncodeDecode(t *testing.T) {
	t.Log("---- FOOTER ENCODE/DECODE TEST ----")

	footer := NewFooter()
	footer.NumDataBlocks = 5
	footer.MinKeyLength = 3
	footer.MaxKeyLength = 10
	footer.TotalRecords = 1234
	footer.MinTimeStamp = utils.Uint128{Low: 1, High: 2}
	footer.MaxTimeStamp = utils.Uint128{Low: 3, High: 4}
	footer.EncodingType = 1
	footer.RestartInterval = 2

	encoded := footer.Encode()
	decoded := &Footer{}
	err := decoded.Decode(encoded)
	if err != nil {
		t.Fatalf("failed to decode footer: %v", err)
	}

	if decoded.NumDataBlocks != footer.NumDataBlocks {
		t.Fatalf("expected NumDataBlocks %d, got %d", footer.NumDataBlocks, decoded.NumDataBlocks)
	}

	if decoded.MinKeyLength != footer.MinKeyLength || decoded.MaxKeyLength != footer.MaxKeyLength {
		t.Fatalf("key length mismatch: expected min=%d max=%d, got min=%d max=%d",
			footer.MinKeyLength, footer.MaxKeyLength, decoded.MinKeyLength, decoded.MaxKeyLength)
	}

	if decoded.TotalRecords != footer.TotalRecords {
		t.Fatalf("expected TotalRecords %d, got %d", footer.TotalRecords, decoded.TotalRecords)
	}

	if decoded.MinTimeStamp != footer.MinTimeStamp || decoded.MaxTimeStamp != footer.MaxTimeStamp {
		t.Fatalf("timestamp mismatch: expected min=%v max=%v, got min=%v max=%v",
			footer.MinTimeStamp, footer.MaxTimeStamp, decoded.MinTimeStamp, decoded.MaxTimeStamp)
	}

	// verify CRC
	dataWithoutCRC := encoded[:FooterSize-4]
	expectedCRC := binary.LittleEndian.Uint32(encoded[FooterSize-4:])
	actualCRC := crc32.ChecksumIEEE(dataWithoutCRC)
	if expectedCRC != actualCRC {
		t.Fatalf("crc mismatch: expected %d, got %d", expectedCRC, actualCRC)
	}
}

func TestFooterValidate(t *testing.T) {
	t.Log("---- FOOTER VALIDATE TEST ----")

	footer := NewFooter()
	footer.Version = 1
	footer.Format = 0
	footer.MagicNumber = MagicNumber

	if err := footer.Validate(); err != nil {
		t.Fatalf("validation failed unexpectedly: %v", err)
	}

	footer.Version = 2
	if err := footer.Validate(); err == nil {
		t.Fatalf("expected error for unsupported version, got nil")
	}
	footer.Version = 1

	footer.Format = 3
	if err := footer.Validate(); err == nil {
		t.Fatalf("expected error for invalid format, got nil")
	}
	footer.Format = 0

	footer.MagicNumber = 0x1234
	if err := footer.Validate(); err == nil {
		t.Fatalf("expected error for invalid magic number, got nil")
	}
}

func TestFooterCRCError(t *testing.T) {
	t.Log("---- FOOTER CRC ERROR TEST ----")

	footer := NewFooter()
	encoded := footer.Encode()

	// corrupt one byte
	encoded[5] ^= 0xFF

	decoded := &Footer{}
	err := decoded.Decode(encoded)
	if err == nil {
		t.Fatalf("expected CRC mismatch error, got nil")
	}
}

func TestFooterWriteReadBuffer(t *testing.T) {
	t.Log("---- FOOTER WRITE/READ BUFFER TEST ----")

	footer := NewFooter()
	footer.NumDataBlocks = 2
	buf := footer.Encode()

	decoded := &Footer{}
	if err := decoded.Decode(buf); err != nil {
		t.Fatalf("failed to decode footer: %v", err)
	}

	if decoded.NumDataBlocks != footer.NumDataBlocks {
		t.Fatalf("expected NumDataBlocks %d, got %d", footer.NumDataBlocks, decoded.NumDataBlocks)
	}
}
