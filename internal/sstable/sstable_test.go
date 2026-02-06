package sstable

import (
	"os"
	"testing"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/utils"
)

func TestSSTableWriteRead(t *testing.T) {
	testFile := "test_sstable.sst"
	defer os.Remove(testFile)
	blockManager := block.NewBlockManager(4096, 100)
	writer, err := NewSSTableWriter(testFile, blockManager, 16, CompressionNone)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	records := []Record{
		{Timestamp: utils.Uint128{High: 0, Low: 1}, Key: []byte("apple"), Value: []byte("red fruit")},
		{Timestamp: utils.Uint128{High: 0, Low: 2}, Key: []byte("banana"), Value: []byte("yellow fruit")},
		{Timestamp: utils.Uint128{High: 0, Low: 3}, Key: []byte("cherry"), Value: []byte("small fruit")},
		{Timestamp: utils.Uint128{High: 0, Low: 4}, Key: []byte("date"), Value: []byte("sweet fruit")},
		{Timestamp: utils.Uint128{High: 0, Low: 5}, Key: []byte("elderberry"), Value: []byte("purple fruit")},
	}
	for _, rec := range records {
		if err := writer.Add(rec); err != nil {
			t.Fatalf("Failed to add record: %v", err)
		}
		t.Logf("  Added: %s -> %s", rec.Key, rec.Value)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}
}
