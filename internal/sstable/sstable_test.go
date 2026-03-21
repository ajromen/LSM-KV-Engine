package sstable

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/data_structures"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
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

//goland:noinspection DuplicatedCode,DuplicatedCode,DuplicatedCode
func TestSSTableWriterBasic(t *testing.T) {
	tempFile := filepath.Join("1.sst")
	defer os.Remove(tempFile)
	cfg := config.NewDefaultConfig()
	blockManager := block.NewBlockManager(cfg.SSTable.DataSegment.BlockSize)
	writer, err := NewSSTableWriter(tempFile, blockManager, 100, 0)
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
	reader, err := NewSSTableReader(1, tempFile, 0, 0)
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
	summaryF, _ := os.Open(tempFile)
	defer summaryF.Close()
	buf := make([]byte, reader.footer.SummaryHandler.Size)
	_, err = summaryF.ReadAt(buf, int64(reader.footer.SummaryHandler.Offset))
	if err != nil {
		t.Fatalf("ReadAt failed: %v", err)
	}
	fmt.Println("SUMMARY RAW BYTES:", buf)
	indexF, _ := os.Open(tempFile)
	defer summaryF.Close()
	buf = make([]byte, reader.footer.IndexHandler.Size)
	_, err = indexF.ReadAt(buf, int64(reader.footer.IndexHandler.Offset))
	if err != nil {
		t.Fatalf("ReadAt failed: %v", err)
	}
	fmt.Println("INDEX RAW BYTES:", buf)
	fmt.Println(reader.footer.TotalRecords)
	indexSegment, _ := DecodeIndexBlock(buf)
	fmt.Println()
	fmt.Println([]byte("key001"))
	fmt.Println("INDEX ENTRIES", indexSegment.Entries)
	fmt.Println("SUMMARY ENTRIES", reader.SummarySegment.Entries)
	fmt.Println("INDEX BLOCK OFFSET", reader.SummarySegment.Entries[0].IndexBlockOffset)
	record, err := reader.Get([]byte("key001"))
	if err != nil {
		t.Fatalf("ReadAt failed: %v", err)
	}
	if record != nil {
		fmt.Println("USPIO GET: ", string(record.Value))
	}
}

//goland:noinspection DuplicatedCode,DuplicatedCode,DuplicatedCode
func TestSSTableMultiFileFormat(t *testing.T) {
	tempFile := filepath.Join(os.TempDir(), "1.sst")
	defer os.Remove(tempFile + ".data")
	defer os.Remove(tempFile + ".filter")
	defer os.Remove(tempFile + ".index")
	defer os.Remove(tempFile + ".summary")
	defer os.Remove(tempFile + ".metadata")
	defer os.Remove(tempFile + ".footer")
	cfg := config.NewDefaultConfig()
	cfg.SSTable.Format = 1
	config.TESTSetSettings(cfg)
	blockManager := block.NewBlockManager(cfg.SSTable.DataSegment.BlockSize)
	writer, err := NewSSTableWriter(tempFile, blockManager, 100, 0)
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
	reader, err := NewSSTableReader(1, tempFile, 1, 0)
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
	summaryF, _ := os.Open(tempFile + ".summary")
	defer summaryF.Close()
	buf := make([]byte, reader.footer.SummaryHandler.Size)
	_, err = summaryF.ReadAt(buf, int64(reader.footer.SummaryHandler.Offset))
	if err != nil {
		t.Fatalf("ReadAt failed: %v", err)
	}
	fmt.Println("SUMMARY RAW BYTES:", buf)
	indexF, _ := os.Open(tempFile + ".index")
	defer summaryF.Close()
	buf = make([]byte, reader.footer.IndexHandler.Size)
	_, err = indexF.ReadAt(buf, int64(reader.footer.IndexHandler.Offset))
	if err != nil {
		t.Fatalf("ReadAt failed: %v", err)
	}
	fmt.Println("INDEX RAW BYTES:", buf)
	fmt.Println(reader.footer.TotalRecords)
	indexSegment, _ := DecodeIndexBlock(buf)
	fmt.Println()
	fmt.Println([]byte("key001"))
	fmt.Println("INDEX ENTRIES", indexSegment.Entries)
	fmt.Println("SUMMARY ENTRIES", reader.SummarySegment.Entries)
	fmt.Println("INDEX BLOCK OFFSET", reader.SummarySegment.Entries[0].IndexBlockOffset)
	record, err := reader.Get([]byte("key009"))
	if err != nil {
		t.Fatalf("ReadAt failed: %v", err)
	}
	if record != nil {
		fmt.Println("USPIO GET: ", string(record.Value))
	}
}

func TestNewManifest_FreshDirectory(t *testing.T) {
	dir, err := os.MkdirTemp("", "manifest-test-*")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := NewManifest(dir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if manifest == nil {
		t.Fatal("expected manifest, got nil")
	}
	if manifest.NextSStableId != 0 {
		t.Errorf("expected NextSStableId=0, got %d", manifest.NextSStableId)
	}
	if len(manifest.Layers) != 0 {
		t.Errorf("expected empty Layers, got %d", len(manifest.Layers))
	}
}

func TestNewManifest_LoadsExisting(t *testing.T) {
	dir, err := os.MkdirTemp("", "manifest-test-*")
	if err != nil {
		t.Fatal(err)
	}

	manifest, err := NewManifest(dir)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	manifest.AddSSTable(SSTableManifest{
		Id:           0,
		BaseFileName: "000000.sst",
		Format:       enums.FormatSingleFile,
		Layer:        0,
	})

	loaded, err := NewManifest(dir)
	if err != nil {
		t.Fatalf("expected no error on reload, got: %v", err)
	}
	if loaded.NextSStableId != manifest.NextSStableId {
		t.Errorf("expected NextSStableId=%d, got %d", manifest.NextSStableId, loaded.NextSStableId)
	}
	if len(loaded.Layers[0]) != 1 {
		t.Errorf("expected 1 sstable in layer 0, got %d", len(loaded.Layers[0]))
	}
	if loaded.Layers[0][0].BaseFileName != "000000.sst" {
		t.Errorf("expected BaseFileName=000000.sst, got %s", loaded.Layers[0][0].BaseFileName)
	}
}

//goland:noinspection DuplicatedCode
func TestSSTableIteratorRaw(t *testing.T) {
	tempFile := filepath.Join("1.sst")
	defer os.Remove(tempFile)
	cfg := config.NewDefaultConfig()
	config.TESTSetSettings(cfg)
	blockManager := block.NewBlockManager(cfg.SSTable.DataSegment.BlockSize)
	writer, err := NewSSTableWriter(tempFile, blockManager, 100, 0)
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
	reader, err := NewSSTableReader(1, tempFile, 0, 0)
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

//goland:noinspection DuplicatedCode
func TestSSTableIterator(t *testing.T) {
	tempFile := filepath.Join("1.sst")
	defer os.Remove(tempFile)
	cfg := config.NewDefaultConfig()
	blockManager := block.NewBlockManager(cfg.SSTable.DataSegment.BlockSize)
	writer, err := NewSSTableWriter(tempFile, blockManager, 100, 0)
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
	reader, err := NewSSTableReader(1, tempFile, 0, 0)
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
	tempFile1 := "1.sst"
	tempFile2 := "2.sst"
	defer os.Remove(tempFile1)
	defer os.Remove(tempFile2)

	cfg := config.NewDefaultConfig()
	blockManager := block.NewBlockManager(cfg.SSTable.DataSegment.BlockSize)

	// SSTable 1 — neparni ključevi
	writer1, err := NewSSTableWriter(tempFile1, blockManager, 100, 0)
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
	writer2, err := NewSSTableWriter(tempFile2, blockManager, 100, 0)
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

	reader1, err := NewSSTableReader(1, tempFile1, 0, 0)
	if err != nil {
		t.Fatalf("Failed to create reader1: %v", err)
	}
	reader2, err := NewSSTableReader(2, tempFile2, 0, 0)
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

//goland:noinspection DuplicatedCode
func TestSSTableMergeIteratorRawWithDuplicates(t *testing.T) {
	tempFile1 := "1.sst"
	tempFile2 := "2.sst"
	defer os.Remove(tempFile1)
	defer os.Remove(tempFile2)
	cfg := config.NewDefaultConfig()
	blockManager := block.NewBlockManager(cfg.SSTable.DataSegment.BlockSize)
	writer1, err := NewSSTableWriter(tempFile1, blockManager, 100, 0)
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
	writer2, err := NewSSTableWriter(tempFile2, blockManager, 100, 0)
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

	reader1, err := NewSSTableReader(1, tempFile1, 0, 0)
	if err != nil {
		t.Fatalf("Failed to create reader1: %v", err)
	}
	reader2, err := NewSSTableReader(2, tempFile2, 0, 0)
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

//goland:noinspection DuplicatedCode
func TestSSTableMergeIteratorWithTombstones(t *testing.T) {
	tempFile1 := "1.sst"
	tempFile2 := "2.sst"
	defer os.Remove(tempFile1)
	defer os.Remove(tempFile2)
	cfg := config.NewDefaultConfig()
	blockManager := block.NewBlockManager(cfg.SSTable.DataSegment.BlockSize)
	writer1, err := NewSSTableWriter(tempFile1, blockManager, 100, 0)
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
	writer2, err := NewSSTableWriter(tempFile2, blockManager, 100, 0)
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
	reader1, err := NewSSTableReader(1, tempFile1, 0, 0)
	if err != nil {
		t.Fatalf("Failed to create reader1: %v", err)
	}
	reader2, err := NewSSTableReader(2, tempFile2, 0, 0)
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
	tempFile := "1.sst"
	defer os.Remove(tempFile)

	cfg := config.NewDefaultConfig()
	blockManager := block.NewBlockManager(cfg.SSTable.DataSegment.BlockSize)

	writer, err := NewSSTableWriter(tempFile, blockManager, 100, 0)
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

	reader, err := NewSSTableReader(1, tempFile, 0, 0)
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
