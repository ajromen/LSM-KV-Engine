package sstable

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
	"github.com/ajromen/LSM-KV-Engine/internal/memtable"
)

type Layer struct {
	SSTables    []*SSTableReader
	sizeBytes   int64
	needsUpdate bool
}

func (l *Layer) Length() int {
	return len(l.SSTables)
}

func (l *Layer) GetSize() int64 {
	if !l.needsUpdate {
		return l.sizeBytes
	}
	var size int64
	for _, reader := range l.SSTables {
		size += reader.SizeBytes
	}
	return size
}

func (l *Layer) AppendSSTable(sstable *SSTableReader) {
	l.SSTables = append(l.SSTables, sstable)
	l.needsUpdate = true
}

func (l *Layer) RemoveSSTable(index int) {
	l.SSTables = append(l.SSTables[:index], l.SSTables[index+1:]...)
}

func newLayer() *Layer {
	return &Layer{
		SSTables:    make([]*SSTableReader, 0),
		sizeBytes:   0,
		needsUpdate: false,
	}
}

type SSTableManager struct {
	Layers       []*Layer
	blockManager *block.BlockManager
	Manifest     *Manifest
	dataDir      string
}

func NewSSTableManager(dataDir string) *SSTableManager {
	manager := &SSTableManager{
		dataDir: dataDir,
		Layers:  make([]*Layer, 0),
	}
	manager.Layers = append(manager.Layers, newLayer())
	blockSize := config.GetSettings().SSTable.DataSegment.BlockSize
	manifest, err := NewManifest(dataDir)
	if err != nil {
		panic(err)
	}
	manager.Manifest = manifest
	manager.blockManager = block.NewBlockManager(blockSize)
	if err := manager.LoadExistingSSTables(); err != nil {
		panic(fmt.Errorf("failed to load existing sstables: %v", err))
	}
	return manager
}

// takes sstable names from Manifest and loads them
// finds max sequence id if not in Manifest
func (sm *SSTableManager) LoadExistingSSTables() error {
	_, err := os.Stat(sm.dataDir)
	if os.IsNotExist(err) {
		err := block.EnsureDir(sm.dataDir)
		if err != nil {
			return fmt.Errorf("cant create data directory: %w", err)
		}
	}
	keys := make([]int, 0, len(sm.Manifest.Layers))
	for k := range sm.Manifest.Layers {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	var maxSeqId uint64
	for _, i := range keys {
		for len(sm.Layers) <= i {
			sm.Layers = append(sm.Layers, newLayer())
		}
		for _, sstManifest := range sm.Manifest.Layers[i] {
			reader, err := NewSSTableReader(sstManifest.Id, sstManifest.BaseFileName, sstManifest.Format, int(sstManifest.Layer))
			if err != nil {
				return fmt.Errorf("cant create SSTable reader: %w", err)
			}
			if sm.Manifest.MaxSeqId == 0 {
				seqId, ok := reader.Metadata.GetUint64(FieldMaxSeqId)
				if !ok {
					return fmt.Errorf("couldn't read metadata segment")
				}
				if maxSeqId < seqId {
					maxSeqId = seqId
				}
			}
			sm.Layers[i].AppendSSTable(reader)
		}
	}
	if sm.Manifest.MaxSeqId == 0 {
		sm.Manifest.MaxSeqId = maxSeqId
		err := sm.Manifest.Save()
		if err != nil {
			return err
		}
	}
	return nil
}

func (sm *SSTableManager) createSSTable(expectedElems uint64, toLayer int) (string, int, *SSTableWriter, error) {
	sstableID := sm.Manifest.NextSStableId
	sm.Manifest.IncrementId()
	filePath := filepath.Join(sm.dataDir, fmt.Sprintf("L%d_%06d%s", toLayer, sstableID, SSTableFileExtension))
	writer, err := NewSSTableWriter(filePath, sm.blockManager, expectedElems, toLayer)
	if err != nil {
		return "", 0, nil, fmt.Errorf("cant create SSTable writer: %w", err)
	}
	return filePath, sstableID, writer, nil
}

func (sm *SSTableManager) addToLayers(reader *SSTableReader, toLayer int) error {
	for len(sm.Layers) <= toLayer {
		sm.Layers = append(sm.Layers, newLayer())
	}
	sm.Layers[toLayer].AppendSSTable(reader)

	sstManifest := SSTableManifest{
		Id:           reader.Id,
		Layer:        uint64(toLayer),
		Format:       config.GetSettings().SSTable.Format,
		BaseFileName: reader.filePath,
	}
	err := sm.Manifest.AddSSTable(sstManifest)
	if err != nil {
		return fmt.Errorf("cant add sstable to Manifest: %w", err)
	}
	return nil
}

// takes memtables and flushes them to sstable
// 1. create new sstable
// 2. add entries
// 3. create reader and add to manager
func (sm *SSTableManager) FlushToSSTable(entries []memtable.MemtableEntry) error {
	if len(entries) == 0 {
		return nil
	}
	fmt.Printf("Flushing %d entries\n", len(entries))
	sort.Slice(entries, func(i, j int) bool { return bytes.Compare(entries[i].Key, entries[j].Key) < 0 })

	// 1. create new sstable
	filePath, sstableID, writer, err := sm.createSSTable(uint64(len(entries)), 0)
	if err != nil {
		return err
	}

	// 2. add entries
	for _, entry := range entries {
		record := Record{
			Key:       entry.Key,
			Value:     entry.Value,
			SeqId:     entry.SeqId,
			Tombstone: entry.Tombstone,
		}
		if err := writer.AddRecord(record); err != nil {
			return fmt.Errorf("cant add record: %w", err)
		}
	}
	if err := writer.Finalize(); err != nil {
		return err
	}

	// 3. create reader and add to manager
	reader, err := NewSSTableReaderFromWriter(writer, sstableID)
	if err != nil {
		return fmt.Errorf("cant create SSTable reader: %w", err)
	}

	err = sm.addToLayers(reader, 0)
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
			sm.Layers[layer].RemoveSSTable(i)
			break
		}
	}

	err := sm.Manifest.RemoveSSTable(id, layer)
	if err != nil {
		return err
	}
	return nil
}

func (sm *SSTableManager) DeleteSSTables(readers []*SSTableReader) error {
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

func (sm *SSTableManager) ClearAll() error {
	for _, layer := range sm.Layers {
		for _, reader := range layer.SSTables {
			if reader != nil && reader.storage != nil {
				reader.storage.Delete()
			}
		}
	}

	sm.Layers = []*Layer{newLayer()}

	if sm.blockManager != nil {
		sm.blockManager.ClearCache()
	}

	sm.Manifest = &Manifest{
		FileDir:       sm.dataDir,
		NextSStableId: 0,
		Layers:        make(map[int][]SSTableManifest),
	}

	return sm.Manifest.Save()
}

// MergeSSTables pass in sstables to merge them into a single sstable and delete old ones
// skipTombstones if it's the last layer
// 1. create new sstable
// 2. iterate through all elems and add to new sstable
// 3. open new reader and add to manager
// 4. delete old sstables
func (sm *SSTableManager) MergeSSTables(readers []*SSTableReader, toLayer int, skipTombstones bool) error {
	// 1. create new sstable

	expectedElems := uint64(0)
	for _, reader := range readers {
		tr, _ := reader.Metadata.GetUint64(FieldTotalRecords)
		expectedElems += tr
	}

	_, sstableID, writer, err := sm.createSSTable(expectedElems, toLayer)
	if err != nil {
		return err
	}

	// 2. iterate through all elems and add to new sstable
	iterator, err := NewSSTableMergeIterator(readers, byte(enums.Heap))
	if err != nil {
		return err
	}
	count := 0
	for iterator.Valid() {
		rec := iterator.Value()
		if rec.Tombstone { // TODO check if record is before checkpoints - check if key exists above
			iterator.Next()
			continue
		}
		err := writer.AddRecord(rec)
		if err != nil {
			return err
		}
		iterator.Next()
		count++
	}
	// last layer all tombstones
	if count == 0 {
		return nil
	}
	//if count == 0 {
	//	return fmt.Errorf("merge produced no records. All %d input SSTables may be empty", len(readers))
	//}
	err = writer.Finalize()
	if err != nil {
		return fmt.Errorf("cant finalize SSTable writer: %w", err)
	}

	// 3. open new reader and add to manager
	reader, err := NewSSTableReaderFromWriter(writer, sstableID)
	if err != nil {
		return err
	}
	err = sm.addToLayers(reader, toLayer)
	if err != nil {
		return err
	}

	// 4. delete old sstables
	err = sm.DeleteSSTables(readers)

	return err
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

	err := sm.Manifest.MoveSSTable(current.Id, fromLayer, toLayer)
	if err != nil {
		return err
	}
	//TODO update footer
	return nil
}
