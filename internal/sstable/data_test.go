package sstable

import (
	"os"
	"testing"

	"github.com/ajromen/LSM-KV-Engine/internal/compressor"
)

func TestWriteReadRecord_NoCompression(t *testing.T) {
	tmp, err := os.CreateTemp("", "datafile_*.bin")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	df := &DataFile{Filename: tmp.Name()}
	file := tmp
	original := &DataRecord{
		CRC:       12345,
		Timestamp: 999,
		Tombstone: false,
		Key:       []byte("user1"),
		Value:     []byte("value1"),
	}
	err = df.writeRecord(file, original, nil)
	if err != nil {
		t.Fatal(err)
	}
	file.Seek(0, 0)
	read, err := df.readRecord(file, nil)
	df.printRecordOnDisk(read, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(read.Key) != string(original.Key) {
		t.Fatalf("key mismatch: %s vs %s", read.Key, original.Key)
	}
	if string(read.Value) != string(original.Value) {
		t.Fatalf("value mismatch: %s vs %s", read.Value, original.Value)
	}
	if read.Timestamp != original.Timestamp {
		t.Fatalf("timestamp mismatch")
	}
	if read.CRC != original.CRC {
		t.Fatalf("crc mismatch")
	}
}

func TestWriteReadRecord_Tombstone(t *testing.T) {
	tmp, err := os.CreateTemp("", "datafile_*.bin")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	df := &DataFile{Filename: tmp.Name()}
	file := tmp
	original := &DataRecord{
		CRC:       1,
		Timestamp: 1,
		Tombstone: true,
		Key:       []byte("deadkey"),
	}
	df.writeRecord(file, original, nil)
	file.Seek(0, 0)
	read, err := df.readRecord(file, nil)
	df.printRecordOnDisk(read, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !read.Tombstone {
		t.Fatal("tombstone not preserved")
	}
	if len(read.Value) != 0 {
		t.Fatal("value should be empty for tombstone")
	}
}

func TestWriteReadRecord_Compression(t *testing.T) {
	tmp, err := os.CreateTemp("", "datafile_*.bin")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	df := &DataFile{Filename: tmp.Name()}
	file := tmp
	compressorDict := compressor.NewCompressorDict()
	compressorDict.Add("user1")
	original := &DataRecord{
		CRC:       12345,
		Timestamp: 999,
		Tombstone: false,
		Key:       []byte("user1"),
		Value:     []byte("value1"),
	}
	df.writeRecord(file, original, compressorDict)
	file.Seek(0, 0)
	read, err := df.readRecord(file, compressorDict)
	df.printRecordOnDisk(read, compressorDict)
	if err != nil {
		t.Fatal(err)
	}
	if string(read.Key) != string(original.Key) && string(read.Key) != "user1" {
		t.Fatalf("key mismatch: %s vs %s", read.Key, original.Key)
	}
	if string(read.Value) != string(original.Value) {
		t.Fatalf("value mismatch: %s vs %s", read.Value, original.Value)
	}
	if read.Timestamp != original.Timestamp {
		t.Fatalf("timestamp mismatch")
	}
	if read.CRC != original.CRC {
		t.Fatalf("crc mismatch")
	}
}
