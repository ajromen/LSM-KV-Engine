package sstable

import (
	"fmt"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/utils"
)

type SSTableReader struct {
	filePath      string
	config        *config.Config
	storage       SegmentStorage
	blockManager  *block.BlockManager
	footer        *Footer
	topLevelIndex *TopLevelIndex
	indexCache    *IndexBlockCache
}

func OpenSSTable(filePath string, blockManager *block.BlockManager, cnfig *config.Config) (*SSTableReader, error) {
	if cnfig == nil {
		cnfig = config.NewDefaultConfig()
	}
	footer, err := ReadFooterFromFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read footer: %w", err)
	}
	if err := footer.Validate(); err != nil {
		return nil, fmt.Errorf("invalid footer: %w", err)
	}
	if footer.Format == 0 {
		cnfig.SSTable.Format = 0
	} else {
		cnfig.SSTable.Format = 1
	}
	storage, err := OpenStorage(filePath, cnfig)
	if err != nil {
		return nil, fmt.Errorf("failed to open storage: %w", err)
	}
	var topLevel *TopLevelIndex
	if footer.IndexHandler.Size > 0 {
		indexData, err := storage.ReadSegment(
			config.SegmentIndex,
			footer.IndexHandler.Offset,
			footer.IndexHandler.Size,
		)
		if err != nil {
			storage.Close()
			return nil, fmt.Errorf("failed to read top level index: %w", err)
		}
		topLevel, err = DecodeTopLevelIndex(indexData)
		if err != nil {
			storage.Close()
			return nil, fmt.Errorf("failed to decode top level index: %w", err)
		}
	}
	return &SSTableReader{
		filePath:      filePath,
		config:        cnfig,
		storage:       storage,
		blockManager:  blockManager,
		footer:        footer,
		topLevelIndex: topLevel,
		indexCache:    NewIndexBlockCache(32),
	}, nil
}

func (sr *SSTableReader) loadIndexBlock(idx int) (*IndexBlock, error) {
	entry := sr.topLevelIndex.Entries[idx]
	cacheKey := fmt.Sprintf("%d", idx)
	if block, ok := sr.indexCache.Get(cacheKey); ok {
		return block, nil
	}
	data, err := sr.storage.ReadSegment(
		config.SegmentIndex,
		entry.Offset,
		entry.Size,
	)
	if err != nil {
		return nil, err
	}
	block, err := DecodeIndexBlock(data)
	if err != nil {
		return nil, err
	}
	sr.indexCache.Put(cacheKey, block)
	return block, nil
}

func (sr *SSTableReader) Get(key []byte) (*Record, bool, error) {
	if sr.topLevelIndex == nil || len(sr.topLevelIndex.Entries) == 0 {
		for blockIdx := uint32(0); blockIdx < sr.footer.NumDataBlocks; blockIdx++ {
			record, found, err := sr.getFromBlock(blockIdx, key)
			if err != nil {
				return nil, false, err
			}
			if found {
				return record, true, nil
			}
		}
		return nil, false, nil
	}
	indexBlockIdx := sr.topLevelIndex.FindIndexBlock(key)
	if indexBlockIdx < 0 {
		return nil, false, nil
	}
	indexBlock, err := sr.loadIndexBlock(indexBlockIdx)
	if err != nil {
		return nil, false, err
	}
	dataBlockIdx := indexBlock.FindBlock(key)
	if dataBlockIdx < 0 {
		return nil, false, nil
	}
	offset := indexBlock.Entries[dataBlockIdx].Offset
	return sr.getFromBlock(uint32(offset), key)
}

func (sr *SSTableReader) getFromBlock(blockIdx uint32, key []byte) (*Record, bool, error) {
	blockKey := block.BlockKey{
		FilePath: sr.filePath,
		Offset:   blockIdx,
	}
	blockData, err := sr.blockManager.Read(blockKey)
	if err != nil {
		return nil, false, err
	}
	blockIterator, err := NewDataBlockIterator(blockData)
	if err != nil {
		return nil, false, err
	}
	defer blockIterator.Close()
	if err := blockIterator.Seek(key); err != nil {
		return nil, false, err
	}
	if blockIterator.Valid() && string(blockIterator.Key()) == string(key) {
		record := &Record{
			Timestamp: blockIterator.Timestamp(),
			Tombstone: blockIterator.Tombstone(),
			Key:       blockIterator.Key(),
			Value:     blockIterator.Value(),
		}
		return record, true, nil
	}
	return nil, false, nil
}

func (sr *SSTableReader) Close() error {
	return nil
}

func (sr *SSTableReader) GetFooter() *Footer {
	return sr.footer
}

type SSTableScanIterator struct {
	reader        *SSTableReader
	startKey      []byte
	endKey        []byte
	currentBlock  uint32
	blockIterator *DataBlockIterator
	valid         bool
}

func (sr *SSTableReader) NewSSTableScanIterator(startKey []byte, endKey []byte) (*SSTableScanIterator, error) {
	iter := &SSTableScanIterator{
		reader:        sr,
		startKey:      startKey,
		endKey:        endKey,
		currentBlock:  0,
		blockIterator: nil,
		valid:         true,
	}
	if sr.footer.NumDataBlocks > 0 {
		if err := iter.loadBlock(); err != nil {
			return nil, fmt.Errorf("failed to load first block: %w", err)
		}
	} else {
		iter.valid = false
	}
	return iter, nil
}

func (sri *SSTableScanIterator) Next() error {
	if !sri.valid {
		return nil
	}
	if sri.blockIterator != nil {
		if err := sri.blockIterator.Next(); err != nil {
			return err
		}
		if sri.blockIterator.Valid() {
			if sri.endKey != nil && string(sri.endKey) < string(sri.blockIterator.Key()) {
				sri.valid = false
				return nil
			}
		}
	}
	sri.currentBlock++
	if sri.currentBlock >= sri.reader.footer.NumDataBlocks {
		sri.valid = false
		return nil
	}
	if err := sri.loadBlock(); err != nil {
		sri.valid = false
		return err
	}
	return nil
}

func (sri *SSTableScanIterator) loadBlock() error {
	blockKey := block.BlockKey{
		FilePath: sri.reader.filePath,
		Offset:   sri.currentBlock,
	}
	blockData, err := sri.reader.blockManager.Read(blockKey)
	if err != nil {
		return err
	}
	iter, err := NewDataBlockIterator(blockData)
	if err != nil {
		return err
	}
	sri.blockIterator = iter
	if sri.currentBlock == 0 && sri.startKey != nil {
		if err := sri.blockIterator.Seek(sri.startKey); err != nil {
			return err
		}
		if !sri.blockIterator.Valid() {
			sri.valid = false
			return nil
		}
	}
	if sri.endKey != nil && sri.blockIterator.Valid() && string(sri.blockIterator.Key()) > string(sri.endKey) {
		sri.valid = false
		return nil
	}
	return nil
}

func (sri *SSTableScanIterator) Key() []byte {
	if sri.blockIterator == nil || !sri.valid {
		return nil
	}
	return sri.blockIterator.Key()
}

func (sri *SSTableScanIterator) Value() []byte {
	if sri.blockIterator == nil || !sri.valid {
		return nil
	}
	return sri.blockIterator.Value()
}

func (sri *SSTableScanIterator) Valid() bool {
	return sri.valid
}

func (sri *SSTableScanIterator) Timestamp() utils.Uint128 {
	if sri.blockIterator == nil || !sri.valid {
		return utils.Uint128{}
	}
	return sri.blockIterator.Timestamp()
}

func (sri *SSTableScanIterator) Tombstone() bool {
	if sri.blockIterator == nil || !sri.valid {
		return false
	}
	return sri.blockIterator.Tombstone()
}

func (sri *SSTableScanIterator) Close() error {
	if sri.blockIterator != nil {
		return sri.blockIterator.Close()
	}
	return nil
}
