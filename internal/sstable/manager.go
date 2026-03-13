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

type SSTableManager struct {
	config        *config.Config
	sstables      []*SSTableReader
	blockManager  *block.BlockManager
	nextSSTableID uint64
	dataDir       string
}

func NewSSTableManager(dataDir string, cfg *config.Config) *SSTableManager {
	manager := &SSTableManager{
		config:        cfg,
		sstables:      make([]*SSTableReader, 0),
		nextSSTableID: 0,
		dataDir:       dataDir,
	}
	blockSize := cfg.SSTable.DataSegment.BlockSize
	manager.blockManager = block.NewBlockManager(blockSize, 100)
	if err := manager.LoadExistingSSTables(); err != nil {
		panic(err)
	}
	return manager
}

func (sm *SSTableManager) LoadExistingSSTables() error {
	files, err := os.ReadDir(sm.dataDir)
	if err != nil {
		return err
	}
	sstableFiles := make(map[string]bool)
	for _, file := range files {
		if !file.IsDir() {
			name := file.Name()
			// Single-file: 000000.sst
			if filepath.Ext(name) == ".sst" && !strings.Contains(strings.TrimSuffix(name, ".sst"), ".") {
				sstableFiles[filepath.Join(sm.dataDir, name)] = true
			}
			// Multi-file: 000000.sst.data -> add 000000.sst to list
			if strings.HasSuffix(name, ".sst.data") {
				basePath := filepath.Join(sm.dataDir, strings.TrimSuffix(name, ".data"))
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
		reader, err := NewSSTableReader(filePath, sm.config)
		if err != nil {
			return err
		}
		sm.sstables = append(sm.sstables, reader)
		var id uint64
		fmt.Sscanf(filepath.Base(filePath), "%d.sst", &id)
		if id >= sm.nextSSTableID {
			sm.nextSSTableID = id + 1
		}
	}
	fmt.Printf("Loaded %d existing SSTables\n", len(sm.sstables))
	return nil
}

func (sm *SSTableManager) FlushToSSTable(entries []memtable.MemtableEntry) error {
	if len(entries) == 0 {
		return nil
	}
	fmt.Printf("Flushing %d entries\n", len(entries))
	sort.Slice(entries, func(i, j int) bool { return bytes.Compare(entries[i].Key, entries[j].Key) < 0 })
	sstableID := sm.nextSSTableID
	sm.nextSSTableID++
	filePath := filepath.Join(sm.dataDir, fmt.Sprintf("%06d.sst", sstableID))
	writer, err := NewSSTableWriter(filePath, sm.blockManager, sm.config, len(entries))
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
	reader, err := NewSSTableReader(filePath, sm.config)
	if err != nil {
		return err
	}
	sm.sstables = append(sm.sstables, reader)
	fmt.Printf("SSTable %s created with %d entries\n", filepath.Base(filePath), len(entries))
	return nil
}

func (sm *SSTableManager) Get(key []byte) ([]byte, bool, error) {
	for i := len(sm.sstables) - 1; i >= 0; i-- {
		record, err := sm.sstables[i].Get(key)
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
