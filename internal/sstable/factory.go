package sstable

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/memtable"
	"github.com/ajromen/LSM-KV-Engine/internal/utils"
)

type SSTableFactory struct {
	config        *config.Config
	sstables      []*SSTableReader
	blockManager  *block.BlockManager
	nextSSTableID uint64
	dataDir       string
}

func NewSSTableFactory(dataDir string, cfg *config.Config) *SSTableFactory {
	factory := &SSTableFactory{
		config:        cfg,
		sstables:      make([]*SSTableReader, 0),
		nextSSTableID: 0,
		dataDir:       dataDir,
	}
	blockSize := cfg.SSTable.DataSegment.BlockSize
	factory.blockManager = block.NewBlockManager(blockSize, 100)
	if err := factory.LoadExistingSSTables(); err != nil {
		panic(err)
	}
	return factory
}

func (f *SSTableFactory) LoadExistingSSTables() error {
	files, err := os.ReadDir(f.dataDir)
	if err != nil {
		return err
	}
	sstableFiles := make(map[string]bool)
	for _, file := range files {
		if !file.IsDir() {
			name := file.Name()
			// Single-file: 000000.sst
			if filepath.Ext(name) == ".sst" && !strings.Contains(strings.TrimSuffix(name, ".sst"), ".") {
				sstableFiles[filepath.Join(f.dataDir, name)] = true
			}
			// Multi-file: 000000.sst.data -> add 000000.sst to list
			if strings.HasSuffix(name, ".sst.data") {
				basePath := filepath.Join(f.dataDir, strings.TrimSuffix(name, ".data"))
				sstableFiles[basePath] = true
			}
		}
	}
	sortedFiles := make([]string, 0, len(sstableFiles))
	for file := range sstableFiles {
		sortedFiles = append(sortedFiles, file)
	}
	sort.Strings(sortedFiles)
	for _, filePath := range sortedFiles {
		reader, err := NewSSTableReader(filePath, f.config)
		if err != nil {
			return err
		}
		f.sstables = append(f.sstables, reader)
		var id uint64
		fmt.Sscanf(filepath.Base(filePath), "%d.sst", &id)
		if id >= f.nextSSTableID {
			f.nextSSTableID = id + 1
		}
	}
	fmt.Printf("Loaded %d existing SSTables\n", len(f.sstables))
	return nil
}

func (f *SSTableFactory) FlushToSSTable(entries []memtable.MemtableEntry) error {
	if len(entries) == 0 {
		return nil
	}
	fmt.Printf("Flushing %d entries\n", len(entries))
	sort.Slice(entries, func(i, j int) bool { return bytes.Compare(entries[i].Key, entries[j].Key) < 0 })
	sstableID := f.nextSSTableID
	f.nextSSTableID++
	filePath := filepath.Join(f.dataDir, fmt.Sprintf("%06d.sst", sstableID))
	writer, err := NewSSTableWriter(filePath, f.blockManager, f.config, len(entries))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		record := Record{
			Key:       entry.Key,
			Value:     entry.Value,
			Timestamp: utils.Uint128{Low: entry.Timestamp},
			Tombstone: entry.Tombstone,
		}
		if err := writer.AddRecord(record); err != nil {
			return err
		}
	}
	if err := writer.Finalize(); err != nil {
		return err
	}
	reader, err := NewSSTableReader(filePath, f.config)
	if err != nil {
		return err
	}
	f.sstables = append(f.sstables, reader)
	fmt.Printf("SSTable %s created with %d entries\n", filepath.Base(filePath), len(entries))
	return nil
}

func (f *SSTableFactory) Get(key []byte) ([]byte, bool, error) {
	for i := len(f.sstables) - 1; i >= 0; i-- {
		record, err := f.sstables[i].Get(key)
		if err != nil {
			return nil, false, err
		}
		if record == nil {
			continue
		}
		return record.Value, true, nil
	}
	return nil, false, nil
}
