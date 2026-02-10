package sstable

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/utils"
)

func cleanupTestFiles(basePath string) {
	os.Remove(basePath)
	os.Remove(basePath + ".filter")
	os.Remove(basePath + ".index")
	os.Remove(basePath + ".summary")
	os.Remove(basePath + ".metadata")
	os.Remove(basePath + ".footer")
}

func TestSSTableSingleFileFormat(t *testing.T) {
	testFile := "test_single_file.sst"
	defer cleanupTestFiles(testFile)
	blockManager := block.NewBlockManager(4096, 100)
	cfg := config.NewDefaultConfig()
	cfg.SSTable.Format = 0
	cfg.SSTable.DataSegment.RestartInterval = 16
	cfg.SSTable.DataSegment.Compression = CompressionNone
	t.Log("=== WRITING SINGLE-FILE FORMAT ===")
	writer, err := NewSSTableWriter(testFile, blockManager, cfg)
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
	t.Logf("Writer stats:")
	t.Logf("  Blocks: %d", writer.numBlocks)
	t.Logf("  Records: %d", writer.numRecords)
	t.Logf("  Format: Single-file (0)")
	if _, err := os.Stat(testFile); err != nil {
		t.Fatalf("Main file should exist: %v", err)
	}
	if _, err := os.Stat(testFile + ".index"); err == nil {
		t.Fatal("Index file should NOT exist in single-file format")
	}
	if _, err := os.Stat(testFile + ".footer"); err == nil {
		t.Fatal("Footer file should NOT exist in single-file format")
	}
	t.Log("\n=== READING SINGLE-FILE FORMAT ===")
	reader, err := OpenSSTable(testFile, blockManager, nil)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()
	footer := reader.GetFooter()
	t.Logf("Footer info:")
	t.Logf("  Format: %d (should be 0 for single-file)", footer.Format)
	t.Logf("  Blocks: %d", footer.NumDataBlocks)
	t.Logf("  Records: %d", footer.TotalRecords)
	t.Logf("  Index offset: %d, size: %d", footer.IndexHandler.Offset, footer.IndexHandler.Size)
	if footer.Format != 0 {
		t.Errorf("Expected format 0 (single-file), got %d", footer.Format)
	}
	record, found, err := reader.Get([]byte("cherry"))
	if err != nil {
		t.Fatalf("Failed to get record: %v", err)
	}
	if !found {
		t.Fatal("Failed to find cherry record")
	}
	if !bytes.Equal([]byte("small fruit"), record.Value) {
		t.Fatalf("Value should be 'small fruit' but is %s", record.Value)
	}
	t.Logf("✓ Successfully read: cherry -> %s", record.Value)
	if reader.indexBlock == nil {
		t.Fatal("Index block should be loaded")
	}
	t.Logf("Index entries: %d", len(reader.indexBlock.Entries))
	for i, entry := range reader.indexBlock.Entries {
		t.Logf("  [%d] %s -> block offset %d", i, entry.Key, entry.Offset)
	}
}

func TestSSTableMultiFileFormat(t *testing.T) {
	testFile := "test_multi_file.sst"
	defer cleanupTestFiles(testFile)
	blockManager := block.NewBlockManager(4096, 100)
	cfg := config.NewDefaultConfig()
	cfg.SSTable.Format = 1
	cfg.SSTable.DataSegment.RestartInterval = 16
	cfg.SSTable.DataSegment.Compression = CompressionNone
	t.Log("=== WRITING MULTI-FILE FORMAT ===")
	writer, err := NewSSTableWriter(testFile, blockManager, cfg)
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
	t.Logf("Writer stats:")
	t.Logf("  Blocks: %d", writer.numBlocks)
	t.Logf("  Records: %d", writer.numRecords)
	t.Logf("  Format: Multi-file (1)")
	if _, err := os.Stat(testFile); err != nil {
		t.Fatalf("Data file should exist: %v", err)
	}
	if _, err := os.Stat(testFile + ".index"); err != nil {
		t.Fatalf("Index file should exist: %v", err)
	}
	if _, err := os.Stat(testFile + ".footer"); err != nil {
		t.Fatalf("Footer file should exist: %v", err)
	}
	dataInfo, _ := os.Stat(testFile)
	indexInfo, _ := os.Stat(testFile + ".index")
	footerInfo, _ := os.Stat(testFile + ".footer")
	t.Logf("File sizes:")
	t.Logf("  Data: %d bytes", dataInfo.Size())
	t.Logf("  Index: %d bytes", indexInfo.Size())
	t.Logf("  Footer: %d bytes", footerInfo.Size())
	t.Log("\n=== READING MULTI-FILE FORMAT ===")
	reader, err := OpenSSTable(testFile, blockManager, nil)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()
	footer := reader.GetFooter()
	t.Logf("Footer info:")
	t.Logf("  Format: %d (should be 1 for multi-file)", footer.Format)
	t.Logf("  Blocks: %d", footer.NumDataBlocks)
	t.Logf("  Records: %d", footer.TotalRecords)
	t.Logf("  Index offset: %d, size: %d", footer.IndexHandler.Offset, footer.IndexHandler.Size)
	if footer.Format != 1 {
		t.Errorf("Expected format 1 (multi-file), got %d", footer.Format)
	}
	record, found, err := reader.Get([]byte("banana"))
	if err != nil {
		t.Fatalf("Failed to get record: %v", err)
	}
	if !found {
		t.Fatal("Failed to find banana record")
	}
	if !bytes.Equal([]byte("yellow fruit"), record.Value) {
		t.Fatalf("Value should be 'yellow fruit' but is %s", record.Value)
	}
	t.Logf("✓ Successfully read: banana -> %s", record.Value)
	if reader.indexBlock == nil {
		t.Fatal("Index block should be loaded")
	}
	t.Logf("Index entries: %d", len(reader.indexBlock.Entries))
	for i, entry := range reader.indexBlock.Entries {
		t.Logf("  [%d] %s -> block offset %d", i, entry.Key, entry.Offset)
	}
}

func TestSSTableMultipleBlocks(t *testing.T) {
	testFile := "test_sstable_multi_blocks.sst"
	defer cleanupTestFiles(testFile)
	blockManager := block.NewBlockManager(4096, 100)
	cfg := config.NewDefaultConfig()
	cfg.SSTable.Format = 0
	cfg.SSTable.DataSegment.RestartInterval = 4
	cfg.SSTable.DataSegment.Compression = CompressionNone
	t.Log("=== WRITING MULTIPLE BLOCKS ===")
	writer, err := NewSSTableWriter(testFile, blockManager, cfg)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	numRecords := 200
	for i := 0; i < numRecords; i++ {
		key := []byte(fmt.Sprintf("key_%05d", i))
		value := []byte(fmt.Sprintf("value_%05d_some_relatively_long_content_to_fill_blocks", i))
		rec := Record{
			Timestamp: utils.Uint128{High: 0, Low: uint64(i)},
			Key:       key,
			Value:     value,
		}
		if err := writer.Add(rec); err != nil {
			t.Fatalf("Failed to add record %d: %v", i, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}
	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	t.Logf("Test Results:")
	t.Logf("  Records: %d", numRecords)
	t.Logf("  Blocks: %d", writer.numBlocks)
	t.Logf("  File size: %d bytes", info.Size())
	t.Logf("  Records/block: %.1f", float64(numRecords)/float64(writer.numBlocks))
	if writer.numBlocks <= 1 {
		t.Log("  Note: Only 1 block created (records fit in single block)")
	} else {
		t.Logf("  ✓ Successfully created %d blocks", writer.numBlocks)
	}
	t.Log("\n=== IN-MEMORY INDEX ===")
	index := writer.DebugIndex()
	t.Logf("Index entries: %d", len(index.Entries))
	for i, entry := range index.Entries {
		t.Logf("  [%02d] key=%s → blockOffset=%d", i, entry.Key, entry.Offset)
	}
	t.Log("\n=== BLOCK VISUALIZATION ===")
	if err := VisualizeDataSegmentFromSSTable(testFile, blockManager); err != nil {
		t.Logf("Visualization error (may be expected): %v", err)
	}
}

func TestPointLookupComparison(t *testing.T) {
	singleFile := "test_lookup_single.sst"
	multiFile := "test_lookup_multi.sst"
	defer cleanupTestFiles(singleFile)
	defer cleanupTestFiles(multiFile)
	blockManager := block.NewBlockManager(4096, 100)
	cfg1 := config.NewDefaultConfig()
	cfg1.SSTable.Format = 0
	writer1, _ := NewSSTableWriter(singleFile, blockManager, cfg1)
	cfg2 := config.NewDefaultConfig()
	cfg2.SSTable.Format = 1
	writer2, _ := NewSSTableWriter(multiFile, blockManager, cfg2)
	for i := 0; i < 50; i++ {
		key := []byte(fmt.Sprintf("key_%03d", i))
		value := []byte(fmt.Sprintf("value_%03d", i))
		rec := Record{
			Timestamp: utils.Uint128{High: 0, Low: uint64(i)},
			Key:       key,
			Value:     value,
		}
		writer1.Add(rec)
		writer2.Add(rec)
	}
	writer1.Close()
	writer2.Close()
	t.Log("=== TESTING POINT LOOKUPS ===")
	reader1, _ := OpenSSTable(singleFile, blockManager, nil)
	defer reader1.Close()
	reader2, _ := OpenSSTable(multiFile, blockManager, nil)
	defer reader2.Close()
	testKeys := []string{"key_010", "key_025", "key_040", "key_999"}
	for _, keyStr := range testKeys {
		key := []byte(keyStr)
		rec1, found1, err1 := reader1.Get(key)
		rec2, found2, err2 := reader2.Get(key)
		if err1 != nil || err2 != nil {
			t.Fatalf("Lookup error for %s: err1=%v, err2=%v", keyStr, err1, err2)
		}
		if found1 != found2 {
			t.Errorf("Inconsistent results for %s: single=%v, multi=%v", keyStr, found1, found2)
		}
		if found1 && found2 {
			if !bytes.Equal(rec1.Value, rec2.Value) {
				t.Errorf("Value mismatch for %s", keyStr)
			}
			t.Logf("✓ %s found in both formats: %s", keyStr, rec1.Value)
		} else if !found1 && !found2 {
			t.Logf("✓ %s not found in either format (expected)", keyStr)
		}
	}
}

func TestIndexPersistence(t *testing.T) {
	testFile := "test_index_persistence.sst"
	defer cleanupTestFiles(testFile)
	blockManager := block.NewBlockManager(4096, 100)
	cfg := config.NewDefaultConfig()
	cfg.SSTable.Format = 0
	t.Log("=== WRITING WITH INDEX ===")
	writer, _ := NewSSTableWriter(testFile, blockManager, cfg)
	for i := 0; i < 100; i++ {
		key := []byte(fmt.Sprintf("key_%05d", i))
		value := []byte(fmt.Sprintf("long_value_%05d_with_some_padding_to_force_multiple_blocks", i))
		rec := Record{
			Timestamp: utils.Uint128{High: 0, Low: uint64(i)},
			Key:       key,
			Value:     value,
		}
		writer.Add(rec)
	}
	indexBeforeClose := writer.DebugIndex()
	t.Logf("Index entries before close: %d", len(indexBeforeClose.Entries))
	writer.Close()
	t.Log("\n=== READING BACK INDEX ===")
	reader, _ := OpenSSTable(testFile, blockManager, nil)
	defer reader.Close()
	if reader.indexBlock == nil {
		t.Fatal("Index block not loaded from file")
	}
	t.Logf("Index entries after reload: %d", len(reader.indexBlock.Entries))
	if len(reader.indexBlock.Entries) != len(indexBeforeClose.Entries) {
		t.Errorf("Index size mismatch: wrote %d, read %d",
			len(indexBeforeClose.Entries), len(reader.indexBlock.Entries))
	}
	for i := 0; i < len(indexBeforeClose.Entries) && i < len(reader.indexBlock.Entries); i++ {
		before := indexBeforeClose.Entries[i]
		after := reader.indexBlock.Entries[i]
		if !bytes.Equal(before.Key, after.Key) {
			t.Errorf("Index entry %d key mismatch: %s vs %s", i, before.Key, after.Key)
		}
		if before.Offset != after.Offset {
			t.Errorf("Index entry %d offset mismatch: %d vs %d", i, before.Offset, after.Offset)
		}
	}
	t.Log("✓ Index persisted and restored correctly")
}
