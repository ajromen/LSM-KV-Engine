package sstable

import (
	"encoding/binary"
	"hash/crc32"
	"os"
	"testing"

	"github.com/ajromen/LSM-KV-Engine/internal/probabilistics"
)

func createTestBloom() *probabilistics.BloomFilter {
	bf := probabilistics.NewBloomFilterWithParams(10, 0.01, nil)
	bf.AddString("apple")
	bf.AddString("banana")
	bf.AddString("cherry")
	return bf
}

func TestFilterSegmentEncodeDecode(t *testing.T) {
	t.Log("---- FILTER SEGMENT ENCODE/DECODE TEST ----")

	bf := createTestBloom()
	segment := NewFilterSegment(bf)

	data, err := segment.Encode()
	if err != nil {
		t.Fatalf("failed to encode filter segment: %v", err)
	}

	decodedSegment, err := DecodeFilterSegment(data)
	if err != nil {
		t.Fatalf("failed to decode filter segment: %v", err)
	}

	if !decodedSegment.Filter().MightContainString("apple") {
		t.Fatalf("decoded filter missing 'apple'")
	}
	if !decodedSegment.Filter().MightContainString("banana") {
		t.Fatalf("decoded filter missing 'banana'")
	}
	if decodedSegment.Filter().MightContainString("notpresent") {
		t.Fatalf("'notpresent' should not exist in filter")
	}

	crcPos := len(data) - 4
	expectedCRC := crc32.ChecksumIEEE(data[:crcPos])
	actualCRC := binary.LittleEndian.Uint32(data[crcPos:])
	if expectedCRC != actualCRC {
		t.Fatalf("crc mismatch: expected %d, got %d", expectedCRC, actualCRC)
	}
}

func TestFilterSegmentCRCError(t *testing.T) {
	t.Log("---- FILTER SEGMENT CRC ERROR TEST ----")

	bf := createTestBloom()
	segment := NewFilterSegment(bf)

	data, err := segment.Encode()
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	data[5] ^= 0xFF

	_, err = DecodeFilterSegment(data)
	if err == nil {
		t.Fatalf("expected CRC mismatch error, got nil")
	}
}

func TestFilterSegmentMightContain(t *testing.T) {
	t.Log("---- FILTER SEGMENT MIGHT CONTAIN TEST ----")

	bf := probabilistics.NewBloomFilterWithParams(5, 0.01, nil)
	bf.AddString("x")
	bf.AddString("y")
	segment := NewFilterSegment(bf)

	if !segment.Filter().MightContainString("x") {
		t.Fatalf("filter should contain 'x'")
	}
	if segment.Filter().MightContainString("z") {
		t.Fatalf("filter should NOT contain 'z'")
	}
}

func TestFilterSegmentWriteReadFile(t *testing.T) {
	t.Log("---- FILTER SEGMENT WRITE/READ FILE TEST ----")

	bf := createTestBloom()
	segment := NewFilterSegment(bf)

	tmpFile, err := os.CreateTemp("", "filterseg_test")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	data, err := segment.Encode()
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	n, err := tmpFile.Write(data)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if n == 0 {
		t.Fatalf("no bytes written")
	}

	if _, err := tmpFile.Seek(0, 0); err != nil {
		t.Fatalf("seek failed: %v", err)
	}
	readData := make([]byte, n)
	if _, err := tmpFile.Read(readData); err != nil {
		t.Fatalf("read failed: %v", err)
	}
	readSegment, err := DecodeFilterSegment(readData)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if !readSegment.Filter().MightContainString("apple") {
		t.Fatalf("read filter missing 'apple'")
	}
	if !readSegment.Filter().MightContainString("banana") {
		t.Fatalf("read filter missing 'banana'")
	}
	if readSegment.Filter().MightContainString("notpresent") {
		t.Fatalf("'notpresent' should not exist in read filter")
	}
}

func TestFilterSegmentNilEncode(t *testing.T) {
	t.Log("---- FILTER SEGMENT NIL ENCODE TEST ----")

	var segment *FilterSegment
	_, err := segment.Encode()
	if err == nil {
		t.Fatalf("expected error encoding nil filter segment, got nil")
	}

	segment = &FilterSegment{}
	_, err = segment.Encode()
	if err == nil {
		t.Fatalf("expected error encoding segment with nil bloom, got nil")
	}
}
