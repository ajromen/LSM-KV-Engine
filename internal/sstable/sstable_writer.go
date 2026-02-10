package sstable

import (
	"errors"
	"fmt"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/utils"
)

type SSTableWriter struct {
	filePath     string
	config       *config.Config
	storage      SegmentStorage
	blockManager *block.BlockManager
	blockBuilder *DataBlockBuilder

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

func NewSSTableWriter(filePath string, blockManager *block.BlockManager, cnfig *config.Config) (*SSTableWriter, error) {
	if cnfig == nil {
		cnfig = config.NewDefaultConfig()
	}
	storage, err := CreateStorage(filePath, cnfig)
	if err != nil {
		return nil, err
	}
	return &SSTableWriter{
		filePath:     filePath,
		config:       cnfig,
		storage:      storage,
		blockManager: blockManager,
		blockBuilder: NewDataBlockBuilder(cnfig.SSTable.DataSegment.RestartInterval, blockManager.BlockSize()),
		numBlocks:    0,
		numRecords:   0,
		minTimestamp: utils.Uint128{High: ^uint64(0), Low: ^uint64(0)},
		maxTimestamp: utils.Uint128{High: 0, Low: 0},
		minKeyLength: ^uint32(0),
		maxKeyLength: 0,
		firstKey:     nil,
		lastKey:      nil,
		indexBlock:   NewIndexBlock(),
		compression:  cnfig.SSTable.DataSegment.Compression,
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
	if sw.blockBuilder.recordCount == 0 {
		return errors.New("no records to write")
	}
	blockData := sw.blockBuilder.Finish(sw.compression)
	_, _, err := sw.storage.WriteSegment(config.SegmentData, blockData)
	if err != nil {
		return err
	}
	blockOffset := sw.numBlocks
	sw.indexBlock.AddFromDataBlock(sw.blockBuilder, blockOffset)
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
	if sw.blockBuilder.RecordCount() > 0 {
		if err := sw.Flush(); err != nil {
			return fmt.Errorf("failed to flush final block: %w", err)
		}
	}
	indexData := sw.indexBlock.EncodeIndexBlock()
	indexOffset, indexSize, err := sw.storage.WriteSegment(config.SegmentIndex, indexData)
	if err != nil {
		return fmt.Errorf("failed to write index block: %w", err)
	}
	footer := NewFooter(sw.config.SSTable)
	footer.NumDataBlocks = sw.numBlocks
	footer.TotalRecords = sw.numRecords
	footer.MinTimeStamp = sw.minTimestamp
	footer.MaxTimeStamp = sw.maxTimestamp
	footer.MinKeyLength = sw.minKeyLength
	footer.MaxKeyLength = sw.maxKeyLength
	footer.IndexHandler = SegmentHandler{
		Offset: indexOffset,
		Size:   indexSize,
	}
	if err := footer.WriteToStorage(sw.storage); err != nil {
		return fmt.Errorf("failed to write footer: %w", err)
	}
	if err := sw.storage.Sync(); err != nil {
		return fmt.Errorf("failed to sync storage: %w", err)
	}
	if err := sw.storage.Close(); err != nil {
		return fmt.Errorf("failed to close storage: %w", err)
	}
	sw.closed = true
	return nil
}

func (sw *SSTableWriter) DebugIndex() *IndexBlock {
	return sw.indexBlock
}
