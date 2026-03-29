package sstable

import (
	"encoding/binary"
	"hash/crc32"
	"testing"
)

func TestFooterEncodeDecode(t *testing.T) {
	t.Log("---- FOOTER ENCODE/DECODE TEST ----")

	footer := NewFooter()

	encoded := footer.Encode()
	decoded := &Footer{}
	err := decoded.Decode(encoded)
	if err != nil {
		t.Fatalf("failed to decode footer: %v", err)
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
	buf := footer.Encode()

	decoded := &Footer{}
	if err := decoded.Decode(buf); err != nil {
		t.Fatalf("failed to decode footer: %v", err)
	}

}
