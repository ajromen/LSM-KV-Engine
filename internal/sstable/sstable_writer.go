package sstable

import (
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/probabilistics"
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

	filterSegment  *FilterSegment
	indexSegment   *IndexSegment
	summarySegment *SummarySegment
	merkleTree     *MerkleTree

	keysForFilter [][]byte

	summarySampling uint32

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
	var filterSegment *FilterSegment
	return &SSTableWriter{
		filePath:        filePath,
		config:          cnfig,
		storage:         storage,
		blockManager:    blockManager,
		blockBuilder:    NewDataBlockBuilder(cnfig.SSTable.DataSegment.RestartInterval, blockManager.BlockSize()),
		numBlocks:       0,
		numRecords:      0,
		minTimestamp:    utils.Uint128{High: ^uint64(0), Low: ^uint64(0)},
		maxTimestamp:    utils.Uint128{High: 0, Low: 0},
		minKeyLength:    ^uint32(0),
		maxKeyLength:    0,
		firstKey:        nil,
		lastKey:         nil,
		keysForFilter:   nil,
		indexSegment:    NewIndexSegment(),
		summarySegment:  nil,
		filterSegment:   filterSegment,
		merkleTree:      NewMerkleTree(),
		summarySampling: 1,
		compression:     cnfig.SSTable.DataSegment.Compression,
		closed:          false,
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
	keyCopy := make([]byte, len(record.Key))
	copy(keyCopy, record.Key)
	sw.keysForFilter = append(sw.keysForFilter, keyCopy)
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
	if sw.merkleTree != nil {
		blockHash := sha256.Sum256(blockData)
		sw.merkleTree.AddLeaf(blockHash)
	}
	_, _, err := sw.storage.WriteSegment(config.SegmentData, blockData)
	if err != nil {
		return err
	}
	logicalBlockIndex := sw.numBlocks
	firstKey := sw.blockBuilder.FirstKey()
	if firstKey != nil {
		indexBlockSize := sw.config.SSTable.IndexSegment.IndexBlockSize
		keyCopy := make([]byte, len(firstKey))
		copy(keyCopy, firstKey)
		sw.indexSegment.AddEntryToBlock(IndexEntry{
			Key:        keyCopy,
			BlockIndex: logicalBlockIndex,
		}, indexBlockSize)
	}
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
	if sw.merkleTree != nil {
		if err := sw.merkleTree.Build(); err != nil {
			return fmt.Errorf("failed to build merkle tree: %w", err)
		}
	}
	expectedElements := uint(sw.numRecords)
	falsePositiveRate := sw.config.ProbabilisticType.BloomFilter.FalsePositiveRate
	bloom := probabilistics.NewBloomFilterWithParams(
		expectedElements,
		float64(falsePositiveRate),
		nil,
	)
	for _, key := range sw.keysForFilter {
		bloom.Add(key)
	}
	sw.filterSegment = NewFilterSegment(bloom)
	sw.keysForFilter = nil
	indexBlockOffsets, indexTotalSize, err := sw.writeIndexSegment()
	if err != nil {
		return fmt.Errorf("failed to write index segment: %w", err)
	}
	sw.summarySegment = BuildSummaryFromIndex(
		indexBlockOffsets, // PHYSICAL offsets!
		sw.indexSegment.Blocks,
		sw.summarySampling,
	)
	summaryData := sw.summarySegment.Encode()
	summaryOffset, summarySize, err := sw.storage.WriteSegment(config.SegmentSummary, summaryData)
	if err != nil {
		return fmt.Errorf("failed to write summary: %w", err)
	}
	var filterOffset uint64
	var filterSize uint32
	if sw.filterSegment != nil {
		filterData, err := sw.filterSegment.Encode()
		if err != nil {
			return fmt.Errorf("failed to encode filter: %w", err)
		}
		filterOffset, filterSize, err = sw.storage.WriteSegment(config.SegmentFilter, filterData)
		if err != nil {
			return fmt.Errorf("failed to write filter: %w", err)
		}
	}
	var metadataOffset uint64
	var metadataSize uint32
	if sw.merkleTree != nil {
		merkleData := sw.merkleTree.Encode()
		metadataOffset, metadataSize, err = sw.storage.WriteSegment(config.SegmentMetadata, merkleData)
		if err != nil {
			return fmt.Errorf("failed to write merkle tree: %w", err)
		}
	}
	var indexStartOffset uint64 = 0
	if len(indexBlockOffsets) > 0 {
		indexStartOffset = indexBlockOffsets[0]
	}
	footer := NewFooter(sw.config.SSTable)
	footer.NumDataBlocks = sw.numBlocks
	footer.TotalRecords = sw.numRecords
	footer.MinTimeStamp = sw.minTimestamp
	footer.MaxTimeStamp = sw.maxTimestamp
	footer.MinKeyLength = sw.minKeyLength
	footer.MaxKeyLength = sw.maxKeyLength
	footer.IndexHandler = SegmentHandler{
		Offset: indexStartOffset,
		Size:   indexTotalSize,
	}
	footer.SummaryHandler = SegmentHandler{
		Offset: summaryOffset,
		Size:   summarySize,
	}
	footer.FilterHandler = SegmentHandler{
		Offset: filterOffset,
		Size:   filterSize,
	}
	footer.MetaDataHandler = SegmentHandler{
		Offset: metadataOffset,
		Size:   metadataSize,
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

func (sw *SSTableWriter) writeIndexSegment() ([]uint64, uint32, error) {
	if len(sw.indexSegment.Blocks) == 0 {
		return nil, 0, nil
	}
	var indexBlockOffsets []uint64
	var currentOffset uint64 = 0
	for _, indexBlock := range sw.indexSegment.Blocks {
		indexBlockData := indexBlock.EncodeIndexBlock()
		offset, size, err := sw.storage.WriteSegment(config.SegmentIndex, indexBlockData)
		if err != nil {
			return nil, 0, err
		}
		indexBlockOffsets = append(indexBlockOffsets, offset)
		currentOffset += uint64(size)
	}
	totalSize := uint32(currentOffset)
	return indexBlockOffsets, totalSize, nil
}

func (sw *SSTableWriter) GetMerkleTree() *MerkleTree {
	return sw.merkleTree
}

func (sw *SSTableWriter) GetIndexSegment() *IndexSegment {
	return sw.indexSegment
}

func (sw *SSTableWriter) GetSummarySegment() *SummarySegment {
	return sw.summarySegment
}

func (sw *SSTableWriter) GetFilterSegment() *FilterSegment {
	return sw.filterSegment
}
