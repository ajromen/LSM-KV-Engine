package sstable

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/memtable"
	"github.com/ajromen/LSM-KV-Engine/internal/utils"
)

type Layer struct {
	sstables []*SSTableReader
}

func newLayer() *Layer {
	return &Layer{
		sstables: make([]*SSTableReader, 0),
	}
}

type SSTableManager struct {
	config       *config.Config
	Layers       []*Layer
	blockManager *block.BlockManager
	manifest     *Manifest
	dataDir      string
}

func NewSSTableManager(dataDir string, cfg *config.Config) *SSTableManager {
	manager := &SSTableManager{
		config:  cfg,
		dataDir: dataDir,
		Layers:  make([]*Layer, 0),
	}
	manager.Layers = append(manager.Layers, newLayer())
	blockSize := cfg.SSTable.DataSegment.BlockSize
	manifest, err := NewManifest(dataDir)
	if err != nil {
		panic(err)
	}
	manager.manifest = manifest
	manager.blockManager = block.NewBlockManager(blockSize, 100)
	if err := manager.LoadExistingSSTables(); err != nil {
		panic(err)
	}
	return manager
}

// takes sstable names from manifest and loads them
func (sm *SSTableManager) LoadExistingSSTables() error {
	_, err := os.Stat(sm.dataDir)
	if os.IsNotExist(err) {
		err := block.EnsureDir(sm.dataDir)
		if err != nil {
			return err
		}
	}
	keys := make([]int, 0, len(sm.manifest.Layers))
	for k := range sm.manifest.Layers {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	for _, i := range keys {
		for len(sm.Layers) <= i {
			sm.Layers = append(sm.Layers, newLayer())
		}
		for _, sstManifest := range sm.manifest.Layers[i] {
			reader, err := NewSSTableReader(sstManifest.BaseFileName, sstManifest.Format, sm.config)
			if err != nil {
				return err
			}
			sm.Layers[i].sstables = append(sm.Layers[i].sstables, reader)
		}
	}
	return nil
}

// takes memtables and flushes them to sstable
func (sm *SSTableManager) FlushToSSTable(entries []memtable.MemtableEntry) error {
	if len(entries) == 0 {
		return nil
	}
	fmt.Printf("Flushing %d entries\n", len(entries))
	sort.Slice(entries, func(i, j int) bool { return bytes.Compare(entries[i].Key, entries[j].Key) < 0 })
	sstableID := sm.manifest.NextSStableId
	sm.manifest.NextSStableId++
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
	reader, err := NewSSTableReader(filePath, sm.config.SSTable.Format, sm.config)
	if err != nil {
		return err
	}
	sm.Layers[0].sstables = append(sm.Layers[0].sstables, reader)

	sstManifest := SSTableManifest{
		Id:           sstableID,
		Layer:        0,
		Format:       sm.config.SSTable.Format,
		BaseFileName: filePath,
	}
	err = sm.manifest.AddSSTable(sstManifest)
	if err != nil {
		return err
	}

	fmt.Printf("SSTable %s created with %d entries\n", filepath.Base(filePath), len(entries))
	return nil
}

func (sm *SSTableManager) Get(key []byte) ([]byte, bool, error) {
	for _, layer := range sm.Layers {
		for i := len(layer.sstables) - 1; i >= 0; i-- {
			record, err := layer.sstables[i].Get(key)
			if err != nil {
				return nil, false, err
			}
			if record == nil {
				continue
			}
			return record.Value, true, nil
		}
	}
	return nil, false, nil
}
