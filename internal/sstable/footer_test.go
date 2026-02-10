package sstable

import (
	"fmt"
	"testing"

	"github.com/ajromen/LSM-KV-Engine/internal/utils"
)

func TestFooterEncodeDecode(t *testing.T) {
	f := NewFooter(1)
	f.FilterHandler = SegmentHandler{Offset: 100, Size: 10}
	f.IndexHandler = SegmentHandler{Offset: 200, Size: 20}
	f.SummaryHandler = SegmentHandler{Offset: 300, Size: 30}
	f.MetaDataHandler = SegmentHandler{Offset: 400, Size: 40}
	f.NumDataBlocks = 5
	f.MinTimeStamp = utils.Uint128{High: 0, Low: 1000}
	f.MaxTimeStamp = utils.Uint128{High: 0, Low: 5000}
	f.MinKeyLength = 3
	f.MaxKeyLength = 20
	f.TotalRecords = 12345
	f.CompressionType = 1
	f.Version = 1
	f.MagicNumber = MagicNumber

	buf := f.Encode()

	var decoded Footer
	if err := decoded.Decode(buf); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Compare important fields
	if decoded.TotalRecords != f.TotalRecords {
		t.Errorf("TotalRecords mismatch: got %d, want %d", decoded.TotalRecords, f.TotalRecords)
	}

	if decoded.MagicNumber != f.MagicNumber {
		t.Errorf("MagicNumber mismatch: got %x, want %x", decoded.MagicNumber, f.MagicNumber)
	}

	fmt.Println("Footer Encode/Decode test passed")
}

func TestFooterCorruption(t *testing.T) {
	f := NewFooter(1)
	buf := f.Encode()

	// Flip one byte to simulate corruption
	buf[0] ^= 0xFF

	var decoded Footer
	err := decoded.Decode(buf)
	if err == nil {
		t.Fatal("Expected Decode to fail due to corruption, but it passed")
	} else {
		fmt.Println("Corruption detected as expected:", err)
	}
}
