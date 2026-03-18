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
	layers       []*Layer
	blockManager *block.BlockManager
	manifest     *Manifest
	dataDir      string
}

func NewSSTableManager(dataDir string, cfg *config.Config) *SSTableManager {
	manager := &SSTableManager{
		config:  cfg,
		dataDir: dataDir,
		layers:  make([]*Layer, 0),
	}
	manager.layers = append(manager.layers, newLayer())
	blockSize := cfg.SSTable.DataSegment.BlockSize
	manifest, err := NewManifest(dataDir)
	if err != nil {
		panic(err)
	}
	manager.manifest = manifest
	manager.blockManager = block.NewBlockManager(blockSize, 100)
	if err := manager.LoadExistingSSTables(); err != nil {
		panic(fmt.Errorf("failed to load existing sstables: %v", err))
	}
	return manager
}

// takes sstable names from manifest and loads them
func (sm *SSTableManager) LoadExistingSSTables() error {
	_, err := os.Stat(sm.dataDir)
	if os.IsNotExist(err) {
		err := block.EnsureDir(sm.dataDir)
		if err != nil {
			return fmt.Errorf("cant create data directory: %w", err)
		}
	}
	keys := make([]int, 0, len(sm.manifest.Layers))
	for k := range sm.manifest.Layers {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	for _, i := range keys {
		for len(sm.layers) <= i {
			sm.layers = append(sm.layers, newLayer())
		}
		for _, sstManifest := range sm.manifest.Layers[i] {
			reader, err := NewSSTableReader(sstManifest.BaseFileName, sstManifest.Format, sm.config)
			if err != nil {
				return fmt.Errorf("cant create SSTable reader: %w", err)
			}
			sm.layers[i].sstables = append(sm.layers[i].sstables, reader)
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
	sm.manifest.IncrementId()
	filePath := filepath.Join(sm.dataDir, fmt.Sprintf("%06d%s", sstableID, SSTableFileExtension))
	writer, err := NewSSTableWriter(filePath, sm.blockManager, sm.config, uint64(len(entries)))
	if err != nil {
		return fmt.Errorf("cant create SSTable writer: %w", err)
	}
	for _, entry := range entries {
		record := Record{
			Key:       entry.Key,
			Value:     entry.Value,
			Timestamp: utils.Uint128{Low: entry.Timestamp},
			Tombstone: entry.Tombstone,
		}
		if err := writer.AddRecord(record); err != nil {
			return fmt.Errorf("cant add record: %w", err)
		}
	}
	if err := writer.Finalize(); err != nil {
		return err
	}
	reader, err := NewSSTableReader(filePath, sm.config.SSTable.Format, sm.config)
	if err != nil {
		return fmt.Errorf("cant create SSTable reader: %w", err)
	}
	sm.layers[0].sstables = append(sm.layers[0].sstables, reader)

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
	for _, layer := range sm.layers {
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

// DeleteSSTable deletes at specified layer/index
func (sm *SSTableManager) DeleteSSTable(layer, id int) error {
	readers := sm.Layers[layer].SSTables
	for i, reader := range readers {
		if reader.Id == id {
			reader.storage.Delete()
			sm.Layers[layer].RemoveSSTable(i)
			break
		}
	}

	err := sm.manifest.RemoveSSTable(id, layer)
	if err != nil {
		return err
	}
	return nil
}

// MergeSSTables pass in sstables to merge them into a single sstable and delete old ones
// 1. create new sstable
// 2. iterate through all elems and add to new sstable
// 3. open new reader and add to manager
// 4. update manifest
// 5. delete old sstables
func (sm *SSTableManager) MergeSSTables(readers []*SSTableReader, toLayer int) error {
	// 1. create new sstable
	sstableID := sm.manifest.NextSStableId
	sm.manifest.IncrementId()
	filePath := filepath.Join(sm.dataDir, fmt.Sprintf("%06d%s", sstableID, SSTableFileExtension))

	expectedElems := uint64(0)
	for _, reader := range readers {
		expectedElems += reader.footer.TotalRecords
	}

	writer, err := NewSSTableWriter(filePath, sm.blockManager, sm.config, expectedElems)
	if err != nil {
		return err
	}

	// 2. iterate through all elems and add to new sstable
	iterator, err := NewSSTableMergeIterator(readers, data_structures.Heap)
	if err != nil {
		return err
	}
	count := 0
	for iterator.Valid() {
		err := writer.AddRecord(iterator.Value())
		if err != nil {
			return err
		}
		iterator.Next()
		count++
	}
	if count == 0 {
		return fmt.Errorf("merge produced no records. All %d input SSTables may be empty", len(readers))
	}
	err = writer.Finalize()
	if err != nil {
		return fmt.Errorf("cant finalize SSTable writer: %w", err)
	}

	// 3. open new reader and add to manager
	reader, err := NewSSTableReader(sstableID, filePath, sm.config.SSTable.Format, sm.config, toLayer)
	if err != nil {
		return err
	}
	for len(sm.Layers) <= toLayer {
		sm.Layers = append(sm.Layers, newLayer())
	}
	sm.Layers[toLayer].AppendSSTable(reader)

	// 4. update manifest
	sstManifest := SSTableManifest{
		Id:           sstableID,
		Layer:        uint64(toLayer),
		Format:       sm.config.SSTable.Format,
		BaseFileName: filePath,
	}
	err = sm.manifest.AddSSTable(sstManifest)
	if err != nil {
		return err
	}

	// 5. delete old sstables
	toDelete := make([]struct{ layer, id int }, len(readers))
	for i, reader := range readers {
		toDelete[i] = struct{ layer, id int }{reader.Layer, reader.Id}
	}
	for _, d := range toDelete {
		if err := sm.DeleteSSTable(d.layer, d.id); err != nil {
			return err
		}
	}

	return nil
}

// MoveSSTable moves sstable from one layer to another
func (sm *SSTableManager) MoveSSTable(current *SSTableReader, toLayer int) error {
	fromLayer := current.Layer

	readers := sm.Layers[fromLayer].SSTables
	for i, r := range readers {
		if r.Id == current.Id {
			sm.Layers[fromLayer].RemoveSSTable(i)
			break
		}
	}
	for len(sm.Layers) <= toLayer {
		sm.Layers = append(sm.Layers, newLayer())
	}

	current.Layer = toLayer
	sm.Layers[toLayer].AppendSSTable(current)

	err := sm.manifest.MoveSSTable(current.Id, fromLayer, toLayer)
	if err != nil {
		return err
	}
	//TODO update footer
	return nil
}
