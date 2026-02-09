package sstable

import (
	"fmt"
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
	/*
		reader, err := OpenSSTable(testFile, blockManager)
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}
		record, found, err := reader.Get([]byte("cherry"))
		if err != nil {
			t.Fatalf("Failed to get record: %v", err)
		}
		if found == false {
			t.Fatalf("Failed to find cherry record")
		}
		if bytes.Compare([]byte("small fruit"), record.Value) != 0 {
			t.Fatalf("Value should be small fruit but is %b", record.Value)
		}
	*/
}

func TestSSTableMultipleBlocks(t *testing.T) {
	testFile := "test_sstable_multi.sst"
	defer os.Remove(testFile)
	blockManager := block.NewBlockManager(4096, 100)
	writer, err := NewSSTableWriter(testFile, blockManager, 4, CompressionNone)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	for i := 0; i < 150; i++ {
		key := []byte(fmt.Sprintf("key_%03d", i))
		value := []byte("some relatively long value to fill the block faster")
		rec := Record{
			Timestamp: utils.Uint128{High: 0, Low: uint64(i)},
			Key:       key,
			Value:     value,
		}
		if err := writer.Add(rec); err != nil {
			t.Fatalf("Failed to add record: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}
	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	expectedSize := int64(writer.numBlocks) * 4096
	t.Logf("  Test Results:")
	t.Logf("  Records: 100")
	t.Logf("  Blocks: %d", writer.numBlocks)
	t.Logf("  File size: %d bytes", info.Size())
	t.Logf("  Expected: %d bytes (%d blocks × 4096)", expectedSize, writer.numBlocks)
	t.Logf("  Records/block: %.1f", float64(100)/float64(writer.numBlocks))
	if info.Size() != expectedSize {
		t.Errorf("FILE SIZE MISMATCH! Got %d, want %d", info.Size(), expectedSize)
	}
	if writer.numBlocks >= 100 {
		t.Errorf("Too many blocks! Should pack multiple records per block")
	}
	if writer.numBlocks <= 1 {
		t.Logf("Note: Only 1 block for 100 records (expected with 4KB blocks)")
	} else {
		t.Logf("Successfully created multiple blocks")
	}
	VisualizeDataSegmentFromSSTable(testFile, blockManager)
	t.Logf("\n================ IN-MEMORY INDEX =================")
	index := writer.DebugIndex()
	t.Logf("Index entries: %d", len(index.Entries))
	for i, entry := range index.Entries {
		t.Logf(
			"  [%02d] key=%q → blockOffset=%d",
			i,
			string(entry.Key),
			entry.Offset,
		)
	}
}
