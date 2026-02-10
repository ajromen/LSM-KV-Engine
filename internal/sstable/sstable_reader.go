package sstable

import (
	"fmt"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/utils"
)

type SSTableReader struct {
	filePath     string
	config       *config.Config
	storage      SegmentStorage
	blockManager *block.BlockManager
	footer       *Footer
	indexBlock   *IndexBlock
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
	var indexBlock *IndexBlock
	if footer.IndexHandler.Size > 0 {
		indexData, err := storage.ReadSegment(config.SegmentIndex, footer.IndexHandler.Offset, footer.IndexHandler.Size)
		if err != nil {
			storage.Close()
			return nil, fmt.Errorf("failed to read index block: %w", err)
		}
		indexBlock, err = DecodeIndexBlock(indexData)
		if err != nil {
			storage.Close()
			return nil, fmt.Errorf("failed to decode index block: %w", err)
		}
	}
	return &SSTableReader{
		filePath:     filePath,
		config:       cnfig,
		storage:      storage,
		blockManager: blockManager,
		footer:       footer,
		indexBlock:   indexBlock,
	}, nil
}

func (sr *SSTableReader) Get(key []byte) (*Record, bool, error) {
	// MAIN FUNCTION FOR GET IN SS TABLE
	// AFTER IMPLEMENTING FILTER THIS WILL BE OPTIMIZED
	// FOR NOW I ONLY LINEAR SCAN THROUGH ALL BLOCKS JUST CHECKING HOW DATA WORKS
	if sr.indexBlock == nil || len(sr.indexBlock.Entries) == 0 {
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
	blockIdx := sr.indexBlock.FindBlock(key)
	if blockIdx < 0 {
		return nil, false, nil
	}
	offset := sr.indexBlock.Entries[blockIdx].Offset
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
	return &SSTableScanIterator{
		reader:        sr,
		startKey:      startKey,
		endKey:        endKey,
		currentBlock:  0,
		blockIterator: nil,
		valid:         true,
	}, nil
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
