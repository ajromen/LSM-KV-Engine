package sstable

import (
	"os"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
)

type SSTableReader struct {
	filePath     string
	blockManager *block.BlockManager
	footer       *Footer
	fileSize     int64
}

func OpenSSTable(filePath string, blockManager *block.BlockManager) (*SSTableReader, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := fileInfo.Size()
	footer := &Footer{} // READ FOOTER SHOULD BE CALLED WAITING TO BE IMPLEMENTED ALSO VALIDATION AND SO
	return &SSTableReader{
		filePath:     filePath,
		blockManager: blockManager,
		footer:       footer,
		fileSize:     fileSize,
	}, nil
}

func (sr *SSTableReader) Get(key []byte) (*Record, bool, error) {
	// MAIN FUNCTION FOR GET IN SS TABLE
	// AFTER IMPLEMENTING FILTER,INDEX... THIS WILL BE OPTIMIZED
	// FOR NOW I ONLY LINEAR SCAN THROUGH ALL BLOCKS JUST CHECKING HOW DATA WORKS
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
	return nil
}
