package sstable

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/data_structures"
	"github.com/ajromen/LSM-KV-Engine/internal/utils"
)

func createTestRecord(key string, value string, timestamp uint64, tombstone bool) Record {
	return Record{
		Timestamp: utils.Uint128{Low: timestamp, High: 0},
		Tombstone: tombstone,
		Key:       []byte(key),
		Value:     []byte(value),
	}
}

func TestSSTableWriterBasic(t *testing.T) {
	tempFile := filepath.Join("test_sstable_basic.sst")
	defer os.Remove(tempFile)
	cfg := config.NewDefaultConfig()
	blockManager := block.NewBlockManager(cfg.SSTable.DataSegment.BlockSize, 100)
	writer, err := NewSSTableWriter(tempFile, blockManager, cfg, 100)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	records := []Record{
		createTestRecord("key001", "value001", 1, false),
		createTestRecord("key002", "value002", 2, false),
		createTestRecord("key003", "value003", 3, false),
		createTestRecord("key004", "value004", 4, false),
		createTestRecord("key005", "value005", 5, false),
		createTestRecord("key006", "value005", 5, false),
		createTestRecord("key007", "value007", 5, false),
		createTestRecord("key008", "value008", 5, false),
		createTestRecord("key009", "value009", 5, false),
		createTestRecord("key010", "value005", 5, false),
		createTestRecord("key011", "value005", 5, false),
		createTestRecord("key012", "value005", 5, false),
		createTestRecord("key013", "value005", 5, false),
		createTestRecord("key014", "value005", 5, false),
		createTestRecord("key015", "value005", 5, false),
		createTestRecord("key016", "value016", 5, false),
		createTestRecord("key017", "value005", 5, false),
	}
	for _, rec := range records {
		if err := writer.AddRecord(rec); err != nil {
			t.Fatalf("Failed to add record: %v", err)
		}
	}
	if err := writer.Finalize(); err != nil {
		t.Fatalf("Failed to finalize: %v", err)
	}
	if _, err := os.Stat(tempFile); os.IsNotExist(err) {
		t.Fatalf("SSTable file was not created")
	}
	reader, err := NewSSTableReader(tempFile, 0, cfg)
	if err != nil {
		fmt.Println("error building reader")
	}
	fmt.Println("INDEX OFFSET:", reader.footer.IndexHandler.Offset)
	fmt.Println("INDEX SIZE:", reader.footer.IndexHandler.Size)
	fmt.Println("SUMMARY OFFSET:", reader.footer.SummaryHandler.Offset)
	fmt.Println("SUMMARY SIZE:", reader.footer.SummaryHandler.Size)
	fmt.Println("FILTER OFFSET:", reader.footer.FilterHandler.Offset)
	fmt.Println("FILTER SIZE:", reader.footer.FilterHandler.Size)
	fmt.Println("METADATA OFFSET:", reader.footer.MetaDataHandler.Offset)
	fmt.Println("METADATA SIZE:", reader.footer.MetaDataHandler.Size)
	fmt.Println("NUMBER OF BLOCKS: ", reader.footer.NumDataBlocks)
	summary_f, _ := os.Open(tempFile)
	defer summary_f.Close()
	buf := make([]byte, reader.footer.SummaryHandler.Size)
	_, err = summary_f.ReadAt(buf, int64(reader.footer.SummaryHandler.Offset))
	if err != nil {
		t.Fatalf("ReadAt failed: %v", err)
	}
	fmt.Println("SUMMARY RAW BYTES:", buf)
	index_f, _ := os.Open(tempFile)
	defer summary_f.Close()
	buf = make([]byte, reader.footer.IndexHandler.Size)
	_, err = index_f.ReadAt(buf, int64(reader.footer.IndexHandler.Offset))
	if err != nil {
		t.Fatalf("ReadAt failed: %v", err)
	}
	fmt.Println("INDEX RAW BYTES:", buf)
	fmt.Println(reader.footer.TotalRecords)
	indexSegment, _ := DecodeIndexBlock(buf)
	fmt.Println()
	fmt.Println([]byte("key001"))
	fmt.Println("INDEX ENTRIES", indexSegment.Entries)
	fmt.Println("SUMMARY ENTRIES", reader.summarySegment.Entries)
	fmt.Println("INDEX BLOCK OFFSET", reader.summarySegment.Entries[0].IndexBlockOffset)
	record, err := reader.Get([]byte("key001"))
	if err != nil {
		t.Fatalf("ReadAt failed: %v", err)
	}
	if record != nil {
		fmt.Println("USPIO GET: ", string(record.Value))
	}
}

func TestSSTableMultiFileFormat(t *testing.T) {
	tempFile := filepath.Join(os.TempDir(), "test_sstable_multifile.sst")
	defer os.Remove(tempFile + ".data")
	defer os.Remove(tempFile + ".filter")
	defer os.Remove(tempFile + ".index")
	defer os.Remove(tempFile + ".summary")
	defer os.Remove(tempFile + ".metadata")
	defer os.Remove(tempFile + ".footer")
	cfg := config.NewDefaultConfig()
	cfg.SSTable.Format = 1
	blockManager := block.NewBlockManager(cfg.SSTable.DataSegment.BlockSize, 100)
	writer, err := NewSSTableWriter(tempFile, blockManager, cfg, 100)
	records := []Record{
		createTestRecord("key001", "value001", 1, false),
		createTestRecord("key002", "value002", 2, false),
		createTestRecord("key003", "value003", 3, false),
		createTestRecord("key004", "value004", 4, false),
		createTestRecord("key005", "value005", 5, false),
		createTestRecord("key006", "value005", 5, false),
		createTestRecord("key007", "value007", 5, false),
		createTestRecord("key008", "value008", 5, false),
		createTestRecord("key009", "value009", 5, false),
		createTestRecord("key010", "value005", 5, false),
		createTestRecord("key011", "value005", 5, false),
		createTestRecord("key012", "value005", 5, false),
		createTestRecord("key013", "value005", 5, false),
		createTestRecord("key014", "value005", 5, false),
		createTestRecord("key015", "value005", 5, false),
		createTestRecord("key016", "value016", 5, false),
		createTestRecord("key017", "value005", 5, false),
	}
	for _, rec := range records {
		if err := writer.AddRecord(rec); err != nil {
			t.Fatalf("Failed to add record: %v", err)
		}
	}
	if err := writer.Finalize(); err != nil {
		t.Fatalf("Failed to finalize: %v", err)
	}
	files := []string{
		tempFile + ".data",
		tempFile + ".filter",
		tempFile + ".index",
		tempFile + ".summary",
		tempFile + ".metadata",
		tempFile + ".footer",
	}
	for _, file := range files {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			t.Errorf("Expected file %s to exist", file)
		}
	}
	reader, err := NewSSTableReader(tempFile, 1, cfg)
	if err != nil {
		t.Fatalf("Failed to create sstable reader: %v", err)
	}
	fmt.Println("INDEX OFFSET:", reader.footer.IndexHandler.Offset)
	fmt.Println("INDEX SIZE:", reader.footer.IndexHandler.Size)
	fmt.Println("SUMMARY OFFSET:", reader.footer.SummaryHandler.Offset)
	fmt.Println("SUMMARY SIZE:", reader.footer.SummaryHandler.Size)
	fmt.Println("FILTER OFFSET:", reader.footer.FilterHandler.Offset)
	fmt.Println("FILTER SIZE:", reader.footer.FilterHandler.Size)
	fmt.Println("METADATA OFFSET:", reader.footer.MetaDataHandler.Offset)
	fmt.Println("METADATA SIZE:", reader.footer.MetaDataHandler.Size)
	fmt.Println("NUMBER OF BLOCKS: ", reader.footer.NumDataBlocks)
	summary_f, _ := os.Open(tempFile + ".summary")
	defer summary_f.Close()
	buf := make([]byte, reader.footer.SummaryHandler.Size)
	_, err = summary_f.ReadAt(buf, int64(reader.footer.SummaryHandler.Offset))
	if err != nil {
		t.Fatalf("ReadAt failed: %v", err)
	}
	fmt.Println("SUMMARY RAW BYTES:", buf)
	index_f, _ := os.Open(tempFile + ".index")
	defer summary_f.Close()
	buf = make([]byte, reader.footer.IndexHandler.Size)
	_, err = index_f.ReadAt(buf, int64(reader.footer.IndexHandler.Offset))
	if err != nil {
		t.Fatalf("ReadAt failed: %v", err)
	}
	fmt.Println("INDEX RAW BYTES:", buf)
	fmt.Println(reader.footer.TotalRecords)
	indexSegment, _ := DecodeIndexBlock(buf)
	fmt.Println()
	fmt.Println([]byte("key001"))
	fmt.Println("INDEX ENTRIES", indexSegment.Entries)
	fmt.Println("SUMMARY ENTRIES", reader.summarySegment.Entries)
	fmt.Println("INDEX BLOCK OFFSET", reader.summarySegment.Entries[0].IndexBlockOffset)
	record, err := reader.Get([]byte("key009"))
	if err != nil {
		t.Fatalf("ReadAt failed: %v", err)
	}
	if record != nil {
		fmt.Println("USPIO GET: ", string(record.Value))
	}
}

func TestSSTableIteratorRaw(t *testing.T) {
	tempFile := filepath.Join("test_sstable_basic.sst")
	defer os.Remove(tempFile)
	cfg := config.NewDefaultConfig()
	blockManager := block.NewBlockManager(cfg.SSTable.DataSegment.BlockSize, 100)
	writer, err := NewSSTableWriter(tempFile, blockManager, cfg, 100)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	records := []Record{
		createTestRecord("key001", "value001", 1, false),
		createTestRecord("key002", "value002", 2, false),
		createTestRecord("key003", "value003", 3, false),
		createTestRecord("key004", "value004", 4, false),
		createTestRecord("key005", "value005", 5, false),
		createTestRecord("key006", "value005", 5, false),
		createTestRecord("key007", "value007", 5, false),
		createTestRecord("key008", "value008", 5, false),
		createTestRecord("key009", "value009", 5, false),
		createTestRecord("key010", "value005", 5, false),
		createTestRecord("key011", "value005", 5, false),
		createTestRecord("key012", "value005", 5, false),
		createTestRecord("key013", "value005", 5, false),
		createTestRecord("key014", "value005", 5, false),
		createTestRecord("key015", "value005", 5, false),
		createTestRecord("key016", "value016", 5, false),
		createTestRecord("key017", "value005", 5, false),
	}
	for _, rec := range records {
		if err := writer.AddRecord(rec); err != nil {
			t.Fatalf("Failed to add record: %v", err)
		}
	}
	if err := writer.Finalize(); err != nil {
		t.Fatalf("Failed to finalize: %v", err)
	}
	if _, err := os.Stat(tempFile); os.IsNotExist(err) {
		t.Fatalf("SSTable file was not created")
	}
	reader, err := NewSSTableReader(tempFile, 0, cfg)
	iterator, err := NewSSTableIteratorRaw(reader)
	if err != nil {
		t.Fatalf("Failed to create iterator: %v", err)
	}
	for iterator.Valid() {
		rec := iterator.Key()
		fmt.Println("RAW:", string(rec.Key), string(rec.Value), rec.Tombstone)
		iterator.Next()
	}
}

func TestSSTableIterator(t *testing.T) {
	tempFile := filepath.Join("test_sstable_basic.sst")
	defer os.Remove(tempFile)
	cfg := config.NewDefaultConfig()
	blockManager := block.NewBlockManager(cfg.SSTable.DataSegment.BlockSize, 100)
	writer, err := NewSSTableWriter(tempFile, blockManager, cfg, 100)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	records := []Record{
		createTestRecord("key001", "value001_new", 2, false),
		createTestRecord("key001", "value001_old", 1, false),
		createTestRecord("key002", "value002_new", 2, false),
		createTestRecord("key002", "value002_old", 1, false),
		createTestRecord("key003", "value003", 1, true),
		createTestRecord("key004", "value004", 1, false),
	}
	for _, rec := range records {
		if err := writer.AddRecord(rec); err != nil {
			t.Fatalf("Failed to add record: %v", err)
		}
	}
	if err := writer.Finalize(); err != nil {
		t.Fatalf("Failed to finalize: %v", err)
	}
	if _, err := os.Stat(tempFile); os.IsNotExist(err) {
		t.Fatalf("SSTable file was not created")
	}
	reader, err := NewSSTableReader(tempFile, 0, cfg)
	iterator, err := NewSSTableIterator(reader)
	if err != nil {
		t.Fatalf("Failed to create iterator: %v", err)
	}
	for iterator.Valid() {
		rec := iterator.Key()
		fmt.Println("RAW:", string(rec.Key), string(rec.Value), rec.Tombstone)
		iterator.Next()
	}
}

func TestSSTableMergeIteratorRaw(t *testing.T) {
	tempFile1 := "test_merge_1.sst"
	tempFile2 := "test_merge_2.sst"
	defer os.Remove(tempFile1)
	defer os.Remove(tempFile2)

	cfg := config.NewDefaultConfig()
	blockManager := block.NewBlockManager(cfg.SSTable.DataSegment.BlockSize, 100)

	// SSTable 1 — neparni ključevi
	writer1, err := NewSSTableWriter(tempFile1, blockManager, cfg, 100)
	if err != nil {
		t.Fatalf("Failed to create writer1: %v", err)
	}
	for _, rec := range []Record{
		createTestRecord("key001", "value001_sst1", 1, false),
		createTestRecord("key003", "value003_sst1", 1, false),
		createTestRecord("key005", "value005_sst1", 1, false),
		createTestRecord("key007", "value007_sst1", 1, false),
		createTestRecord("key009", "value007_sst1", 1, false),
		createTestRecord("key011", "value007_sst1", 1, false),
	} {
		if err := writer1.AddRecord(rec); err != nil {
			t.Fatalf("Failed to add record to writer1: %v", err)
		}
	}
	if err := writer1.Finalize(); err != nil {
		t.Fatalf("Failed to finalize writer1: %v", err)
	}

	// SSTable 2 — parni ključevi
	writer2, err := NewSSTableWriter(tempFile2, blockManager, cfg, 100)
	if err != nil {
		t.Fatalf("Failed to create writer2: %v", err)
	}
	for _, rec := range []Record{
		createTestRecord("key002", "value002_sst2", 1, false),
		createTestRecord("key004", "value004_sst2", 1, false),
		createTestRecord("key006", "value006_sst2", 1, false),
		createTestRecord("key008", "value008_sst2", 1, false),
		createTestRecord("key010", "value007_sst1", 1, false),
		createTestRecord("key012", "value007_sst1", 1, false),
	} {
		if err := writer2.AddRecord(rec); err != nil {
			t.Fatalf("Failed to add record to writer2: %v", err)
		}
	}
	if err := writer2.Finalize(); err != nil {
		t.Fatalf("Failed to finalize writer2: %v", err)
	}

	reader1, err := NewSSTableReader(tempFile1, 0, cfg)
	if err != nil {
		t.Fatalf("Failed to create reader1: %v", err)
	}
	reader2, err := NewSSTableReader(tempFile2, 0, cfg)
	if err != nil {
		t.Fatalf("Failed to create reader2: %v", err)
	}

	iterator, err := NewSSTableMergeIteratorRaw([]*SSTableReader{reader1, reader2}, data_structures.Heap)
	if err != nil {
		t.Fatalf("Failed to create merge iterator: %v", err)
	}

	fmt.Println("=== MergeIteratorRaw: interleaved keys ===")
	for iterator.Valid() {
		rec := iterator.Key()
		fmt.Println("RAW:", string(rec.Key), string(rec.Value), rec.Tombstone)
		iterator.Next()
	}
}

func TestSSTableMergeIteratorRawWithDuplicates(t *testing.T) {
	tempFile1 := "test_merge_dup_1.sst"
	tempFile2 := "test_merge_dup_2.sst"
	defer os.Remove(tempFile1)
	defer os.Remove(tempFile2)
	cfg := config.NewDefaultConfig()
	blockManager := block.NewBlockManager(cfg.SSTable.DataSegment.BlockSize, 100)
	writer1, err := NewSSTableWriter(tempFile1, blockManager, cfg, 100)
	if err != nil {
		t.Fatalf("Failed to create writer1: %v", err)
	}
	for _, rec := range []Record{
		createTestRecord("key001", "value001_sst1", 1, false),
		createTestRecord("key003", "value003_sst1", 1, false),
		createTestRecord("key005", "value005_sst1", 1, false),
		createTestRecord("key007", "value007_sst1", 1, false),
		createTestRecord("key009", "value007_sst1", 1, false),
		createTestRecord("key011", "value007_sst1", 1, false),
	} {
		if err := writer1.AddRecord(rec); err != nil {
			t.Fatalf("Failed to add record: %v", err)
		}
	}
	if err := writer1.Finalize(); err != nil {
		t.Fatalf("Failed to finalize writer1: %v", err)
	}
	writer2, err := NewSSTableWriter(tempFile2, blockManager, cfg, 100)
	if err != nil {
		t.Fatalf("Failed to create writer2: %v", err)
	}
	for _, rec := range []Record{
		createTestRecord("key002", "value002_sst2", 1, false),
		createTestRecord("key004", "value004_sst2", 1, false),
		createTestRecord("key006", "value006_sst2", 1, false),
		createTestRecord("key008", "value008_sst2", 1, false),
		createTestRecord("key010", "value007_sst1", 1, false),
		createTestRecord("key012", "value007_sst1", 1, false),
	} {
		if err := writer2.AddRecord(rec); err != nil {
			t.Fatalf("Failed to add record: %v", err)
		}
	}
	if err := writer2.Finalize(); err != nil {
		t.Fatalf("Failed to finalize writer2: %v", err)
	}

	reader1, err := NewSSTableReader(tempFile1, 0, cfg)
	if err != nil {
		t.Fatalf("Failed to create reader1: %v", err)
	}
	reader2, err := NewSSTableReader(tempFile2, 0, cfg)
	if err != nil {
		t.Fatalf("Failed to create reader2: %v", err)
	}
	fmt.Println("=== MergeIteratorRaw: duplicates (both versions visible) ===")
	iterRaw, err := NewSSTableMergeIteratorRaw([]*SSTableReader{reader1, reader2}, data_structures.Heap)
	if err != nil {
		t.Fatalf("Failed to create raw merge iterator: %v", err)
	}
	for iterRaw.Valid() {
		rec := iterRaw.Key()
		fmt.Println("RAW:", string(rec.Key), string(rec.Value), "ts:", rec.Timestamp.Low)
		iterRaw.Next()
	}
	fmt.Println("=== MergeIterator: duplicates (only newest visible) ===")
	iterFiltered, err := NewSSTableMergeIterator([]*SSTableReader{reader1, reader2}, data_structures.Heap)
	if err != nil {
		t.Fatalf("Failed to create filtered merge iterator: %v", err)
	}
	for iterFiltered.Valid() {
		rec := iterFiltered.Key()
		fmt.Println("FILTERED:", string(rec.Key), string(rec.Value), "ts:", rec.Timestamp.Low)
		iterFiltered.Next()
	}
}

func TestSSTableMergeIteratorWithTombstones(t *testing.T) {
	tempFile1 := "test_merge_tomb_1.sst"
	tempFile2 := "test_merge_tomb_2.sst"
	defer os.Remove(tempFile1)
	defer os.Remove(tempFile2)
	cfg := config.NewDefaultConfig()
	blockManager := block.NewBlockManager(cfg.SSTable.DataSegment.BlockSize, 100)
	writer1, err := NewSSTableWriter(tempFile1, blockManager, cfg, 100)
	if err != nil {
		t.Fatalf("Failed to create writer1: %v", err)
	}
	for _, rec := range []Record{
		createTestRecord("key001", "value001_sst1", 1, false),
		createTestRecord("key003", "value003_sst1", 1, false),
		createTestRecord("key005", "value005_sst1", 1, false),
		createTestRecord("key007", "value007_sst1", 1, false),
		createTestRecord("key009", "value007_sst1", 1, false),
		createTestRecord("key011", "value007_sst1", 1, false),
	} {
		if err := writer1.AddRecord(rec); err != nil {
			t.Fatalf("Failed to add record: %v", err)
		}
	}
	if err := writer1.Finalize(); err != nil {
		t.Fatalf("Failed to finalize writer1: %v", err)
	}
	writer2, err := NewSSTableWriter(tempFile2, blockManager, cfg, 100)
	if err != nil {
		t.Fatalf("Failed to create writer2: %v", err)
	}
	for _, rec := range []Record{
		createTestRecord("key002", "value002_sst2", 1, false),
		createTestRecord("key004", "value004_sst2", 1, false),
		createTestRecord("key006", "value006_sst2", 1, false),
		createTestRecord("key008", "value008_sst2", 1, false),
		createTestRecord("key010", "value007_sst1", 1, false),
		createTestRecord("key012", "value007_sst1", 1, false),
	} {
		if err := writer2.AddRecord(rec); err != nil {
			t.Fatalf("Failed to add record: %v", err)
		}
	}
	if err := writer2.Finalize(); err != nil {
		t.Fatalf("Failed to finalize writer2: %v", err)
	}
	reader1, err := NewSSTableReader(tempFile1, 0, cfg)
	if err != nil {
		t.Fatalf("Failed to create reader1: %v", err)
	}
	reader2, err := NewSSTableReader(tempFile2, 0, cfg)
	if err != nil {
		t.Fatalf("Failed to create reader2: %v", err)
	}
	fmt.Println("=== MergeIteratorRaw: tombstones visible ===")
	iterRaw, err := NewSSTableMergeIteratorRaw([]*SSTableReader{reader1, reader2}, data_structures.Heap)
	if err != nil {
		t.Fatalf("Failed to create raw merge iterator: %v", err)
	}
	for iterRaw.Valid() {
		rec := iterRaw.Key()
		fmt.Println("RAW:", string(rec.Key), string(rec.Value), "tombstone:", rec.Tombstone, "ts:", rec.Timestamp.Low)
		iterRaw.Next()
	}
	fmt.Println("=== MergeIterator: tombstones filtered out ===")
	iterFiltered, err := NewSSTableMergeIterator([]*SSTableReader{reader1, reader2}, data_structures.Heap)
	if err != nil {
		t.Fatalf("Failed to create filtered merge iterator: %v", err)
	}
	for iterFiltered.Valid() {
		rec := iterFiltered.Key()
		fmt.Println("FILTERED:", string(rec.Key), string(rec.Value), "ts:", rec.Timestamp.Low)
		iterFiltered.Next()
	}
}

func TestSSTableGet(t *testing.T) {
	tempFile := "test_sstable_get.sst"
	defer os.Remove(tempFile)

	cfg := config.NewDefaultConfig()
	blockManager := block.NewBlockManager(cfg.SSTable.DataSegment.BlockSize, 100)

	writer, err := NewSSTableWriter(tempFile, blockManager, cfg, 100)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	records := []Record{
		createTestRecord("key001", "value001_sst1", 1, false),
		createTestRecord("key003", "value003_sst1", 1, false),
		createTestRecord("key005", "value005_sst1", 1, false),
		createTestRecord("key007", "value007_sst1", 1, false),
		createTestRecord("key009", "value009_sst1", 1, false),
		createTestRecord("key011", "value011_sst1", 1, false),
	}

	for _, rec := range records {
		if err := writer.AddRecord(rec); err != nil {
			t.Fatalf("Failed to add record: %v", err)
		}
	}

	if err := writer.Finalize(); err != nil {
		t.Fatalf("Failed to finalize writer: %v", err)
	}

	reader, err := NewSSTableReader(tempFile, 0, cfg)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}

	tests := []struct {
		key   string
		value string
	}{
		{"key001", "value001_sst1"},
		{"key003", "value003_sst1"},
		{"key005", "value005_sst1"},
		{"key007", "value007_sst1"},
		{"key009", "value009_sst1"},
	}

	for _, tt := range tests {
		rec, err := reader.Get([]byte(tt.key))
		if err != nil {
			t.Fatalf("Get failed for key %s: %v", tt.key, err)
		}

		if rec == nil {
			t.Fatalf("Key %s not found", tt.key)
		}

		if string(rec.Value) != tt.value {
			t.Fatalf("Expected %s, got %s", tt.value, string(rec.Value))
		}
	}

	// test key that does not exist
	rec, err := reader.Get([]byte("key999"))
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if rec != nil {
		t.Fatalf("Expected nil for non-existing key")
	}
}
