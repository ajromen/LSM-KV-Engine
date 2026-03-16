package sstable

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/data_structures"
	"github.com/ajromen/LSM-KV-Engine/internal/memtable"
	"github.com/ajromen/LSM-KV-Engine/internal/utils"
)

type Layer struct {
	SSTables []*SSTableReader
}

func (l *Layer) Length() int {
	return len(l.SSTables)
}

func (l *Layer) GetSize() int64 {
	var size int64
	for _, reader := range l.SSTables {
		size += reader.SizeBytes
	}
	return size
}

func newLayer() *Layer {
	return &Layer{
		SSTables: make([]*SSTableReader, 0),
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
			reader, err := NewSSTableReader(sstManifest.Id, sstManifest.BaseFileName, sstManifest.Format, sm.config, int(sstManifest.Layer))
			if err != nil {
				return err
			}
			sm.Layers[i].SSTables = append(sm.Layers[i].SSTables, reader)
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
	filePath := filepath.Join(sm.dataDir, fmt.Sprintf("%06d.sst", sstableID))
	writer, err := NewSSTableWriter(filePath, sm.blockManager, sm.config, uint64(len(entries)))
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
	reader, err := NewSSTableReader(sstableID, filePath, sm.config.SSTable.Format, sm.config, 0)
	if err != nil {
		return err
	}
	sm.Layers[0].SSTables = append(sm.Layers[0].SSTables, reader)

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
		for i := len(layer.SSTables) - 1; i >= 0; i-- {
			record, err := layer.SSTables[i].Get(key)
			if err != nil {
				return nil, false, err
			}
			if record == nil {
				continue
			}
			if record.Tombstone {
				return nil, false, nil
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
			sm.Layers[layer].SSTables = append(sm.Layers[layer].SSTables[:i], sm.Layers[layer].SSTables[i+1:]...)
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
	filePath := filepath.Join(sm.dataDir, fmt.Sprintf("%06d.sst", sstableID))

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
	for iterator.Valid() {
		err := writer.AddRecord(iterator.Value())
		if err != nil {
			return err
		}
		iterator.Next()
	}
	err = writer.Finalize()
	if err != nil {
		return err
	}

	// 3. open new reader and add to manager
	reader, err := NewSSTableReader(sstableID, filePath, sm.config.SSTable.Format, sm.config, toLayer)
	if err != nil {
		return err
	}
	for len(sm.Layers) <= toLayer {
		sm.Layers = append(sm.Layers, newLayer())
	}
	sm.Layers[toLayer].SSTables = append(sm.Layers[toLayer].SSTables, reader)

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
			sm.Layers[fromLayer].SSTables = append(readers[:i], readers[i+1:]...)
			break
		}
	}
	for len(sm.Layers) <= toLayer {
		sm.Layers = append(sm.Layers, newLayer())
	}

	current.Layer = toLayer
	sm.Layers[toLayer].SSTables = append(sm.Layers[toLayer].SSTables, current)

	err := sm.manifest.MoveSSTable(current.Id, fromLayer, toLayer)
	if err != nil {
		return err
	}
	//TODO update footer
	return nil
}
