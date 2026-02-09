package sstable

import (
	"errors"
	"fmt"
	"os"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/utils"
)

type SSTableWriter struct {
	filePath     string
	file         *os.File
	blockManager *block.BlockManager
	blockBuilder *DataBlockBuilder
	blockOffset  uint32

	numBlocks    uint32
	numRecords   uint64
	minTimestamp utils.Uint128
	maxTimestamp utils.Uint128
	minKeyLength uint32
	maxKeyLength uint32
	firstKey     []byte
	lastKey      []byte

	indexBlock *IndexBlock

	closed      bool
	compression byte
}

func NewSSTableWriter(filePath string, blockManager *block.BlockManager, restartInterval int, compression byte) (*SSTableWriter, error) {
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return nil, err
	}
	return &SSTableWriter{
		filePath:     filePath,
		file:         file,
		blockManager: blockManager,
		blockBuilder: NewDataBlockBuilder(restartInterval, blockManager.BlockSize()),
		blockOffset:  0,
		numBlocks:    0,
		numRecords:   0,
		minTimestamp: utils.Uint128{High: ^uint64(0), Low: ^uint64(0)},
		maxTimestamp: utils.Uint128{High: 0, Low: 0},
		minKeyLength: ^uint32(0),
		maxKeyLength: 0,
		firstKey:     nil,
		lastKey:      nil,
		indexBlock:   NewIndexBlock(),
		compression:  compression,
		closed:       false,
	}, nil
}

func (sw *SSTableWriter) Add(record Record) error {
	if sw.closed {
		return errors.New("SSTableWriter is already closed")
	}
	if sw.lastKey != nil && string(record.Key) <= string(sw.lastKey) {
		return errors.New("Keys must be sorted")
	}
	sw.updateStats(record)
	if !sw.blockBuilder.AddRecord(record) {
		if err := sw.Flush(); err != nil {
			return err
		}
		if !sw.blockBuilder.AddRecord(record) {
			return errors.New("record too large for empty bloc")
		}
	}
	return nil
}

func (sw *SSTableWriter) Flush() error {
	if sw.file == nil {
		return nil
	}
	if sw.blockBuilder.recordCount == 0 {
		return errors.New("no records to write")
	}
	blockData := sw.blockBuilder.Finish(sw.compression)
	blockKey := block.BlockKey{
		FilePath: sw.filePath,
		Offset:   sw.blockOffset,
	}
	if err := sw.blockManager.WriteAt(sw.file, blockKey, blockData); err != nil {
		return err
	}
	sw.indexBlock.AddFromDataBlock(sw.blockBuilder, sw.blockOffset)
	sw.blockOffset++
	sw.numBlocks++
	sw.blockBuilder.Reset()
	return nil
}

func (sw *SSTableWriter) updateStats(record Record) {
	sw.numRecords++
	if sw.firstKey == nil {
		sw.firstKey = make([]byte, len(record.Key))
		copy(sw.firstKey, record.Key)
	}
	sw.lastKey = make([]byte, len(record.Key))
	copy(sw.lastKey, record.Key)
	if utils.Uint128LT(record.Timestamp, sw.minTimestamp) {
		sw.minTimestamp = record.Timestamp
	}
	if utils.Uint128GE(record.Timestamp, sw.maxTimestamp) {
		sw.maxTimestamp = record.Timestamp
	}
	keyLen := uint32(len(record.Key))
	if keyLen < sw.minKeyLength {
		sw.minKeyLength = keyLen
	}
	if keyLen > sw.maxKeyLength {
		sw.maxKeyLength = keyLen
	}
}

func (sw *SSTableWriter) Close() error {
	if sw.closed {
		return nil
	}
	if err := sw.Flush(); err != nil {
		return fmt.Errorf("failed to flush final block: %w", err)
	}
	if _, err := sw.indexBlock.WriteToFile(sw.file); err != nil {
		return fmt.Errorf("failed to write index block: %w", err)
	}
	if err := sw.file.Close(); err != nil {
		return fmt.Errorf("failed to close file: %w", err)
	}
	sw.closed = true
	return nil
}

func (sw *SSTableWriter) DebugIndex() *IndexBlock {
	return sw.indexBlock
}
