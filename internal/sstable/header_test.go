package sstable

import (
	"os"
	"testing"
)

func TestHeaderWriteRead(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "header_test")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	h1 := &Header{
		HasCompression:    true,
		DataOffset:        100,
		DataSize:          500,
		FilterOffset:      600,
		FilterSize:        50,
		IndexOffset:       700,
		IndexSize:         30,
		SummaryOffset:     800,
		SummarySize:       20,
		MetaDataOffset:    900,
		MetaDataSize:      10,
		CompressionOffset: 1000,
		CompressionSize:   5,
	}

	err = h1.WriteHeader(tmpFile, h1)
	if err != nil {
		t.Fatalf("WriteHeader failed: %v", err)
	}

	_, err = tmpFile.Seek(0, 0)
	if err != nil {
		t.Fatalf("failed to seek file: %v", err)
	}

	h2, err := h1.readHeader(tmpFile)
	if err != nil {
		t.Fatalf("readHeader failed: %v", err)
	}

	if *h1 != *h2 {
		t.Errorf("headers do not match!\nexpected: %+v\ngot: %+v", *h1, *h2)
	}
}
