package sstable

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
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
		createTestRecord("key009", "value005", 5, false),
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
	reader, err := NewSSTableReader(tempFile, blockManager, cfg)
	if err != nil {
		t.Fatalf("Failed to create sstable reader: %v", err)
	}
	info, _ := os.Stat(tempFile)
	fmt.Println("FILE SIZE:", info.Size())
	fmt.Println("SUMMARY OFFSET:", reader.footer.SummaryHandler.Offset)
	fmt.Println("SUMMARY SIZE:", reader.footer.SummaryHandler.Size)
	fmt.Println("NUMBER OF BLOCKS: ", reader.footer.NumDataBlocks)
	f, _ := os.Open(tempFile)
	defer f.Close()
	buf := make([]byte, reader.footer.SummaryHandler.Size)
	_, err = f.ReadAt(buf, int64(reader.footer.SummaryHandler.Offset))
	if err != nil {
		t.Fatalf("ReadAt failed: %v", err)
	}
	fmt.Println("SUMMARY RAW BYTES:", buf)
	buf = make([]byte, reader.footer.IndexHandler.Size)
	_, err = f.ReadAt(buf, int64(reader.footer.IndexHandler.Offset))
	if err != nil {
		t.Fatalf("ReadAt failed: %v", err)
	}
	fmt.Println("INDEX RAW BYTES:", buf)
	fmt.Println(reader.footer.TotalRecords)
	fmt.Println(reader.footer.SummaryHandler.Size)
	fmt.Println(reader.footer.SummaryHandler.Offset)
	fmt.Println(reader.footer.IndexHandler.Size)
	fmt.Println(reader.footer.IndexHandler.Offset)
	fmt.Println("SUMMARY ENTRIES", reader.summarySegment.Entries)
	fmt.Println("INDEX BLOCK OFFSET", reader.summarySegment.Entries[0].IndexBlockOffset)
	record, err := reader.Get([]byte("key020"))
	if err != nil {
		t.Fatalf("ReadAt failed: %v", err)
	}
	if record != nil {
		fmt.Println("USPIO GET: ", string(record.Value))
	}
}

func TestSSTableMultiFileFormat(t *testing.T) {
	tempFile := filepath.Join(os.TempDir(), "test_sstable_multifile.db")
	defer os.Remove(tempFile)
	defer os.Remove(tempFile + ".filter")
	defer os.Remove(tempFile + ".index")
	defer os.Remove(tempFile + ".summary")
	defer os.Remove(tempFile + ".metadata")
	defer os.Remove(tempFile + ".footer")
	cfg := config.NewDefaultConfig()
	cfg.SSTable.Format = 1
	blockManager := block.NewBlockManager(cfg.SSTable.DataSegment.BlockSize, 100)
	writer, err := NewSSTableWriter(tempFile, blockManager, cfg, 100)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	for i := 0; i < 20; i++ {
		key := fmt.Sprintf("key%03d", i)
		value := fmt.Sprintf("value%03d", i)
		rec := createTestRecord(key, value, uint64(i), false)
		writer.AddRecord(rec)
	}
	writer.Finalize()
	files := []string{
		tempFile,
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
}
