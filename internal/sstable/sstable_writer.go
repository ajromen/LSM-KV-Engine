package sstable

import (
	"bytes"
	"errors"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/probabilistics"
	"github.com/ajromen/LSM-KV-Engine/internal/utils"
)

type SSTableWriter struct {
	storage           SegmentStorage
	blockManager      *block.BlockManager
	config            config.SSTableConfig
	filePath          string
	dataBlockBuilder  *DataBlockBuilder
	indexSegment      *IndexSegment
	summarySegment    *SummarySegment
	filterSegment     *FilterSegment
	merkleTree        *MerkleTree
	footer            *Footer
	currentBlockIndex uint32
	recordCount       uint64
	minTimestamp      utils.Uint128
	maxTimestamp      utils.Uint128
	minKeyLength      uint32
	maxKeyLength      uint32
	minKey            []byte
	maxKey            []byte
	firstRecord       bool
}

func NewSSTableWriter(filePath string, blockManager *block.BlockManager, cfg *config.Config, expectedElements int) (*SSTableWriter, error) {
	storage, err := CreateStorage(filePath, cfg, blockManager)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		cfg = config.NewDefaultConfig()
	}
	var filterSegment *FilterSegment
	if expectedElements > 0 {
		filter := probabilistics.NewBloomFilterWithParams(uint(expectedElements), float64(cfg.ProbabilisticType.BloomFilter.FalsePositiveRate), nil)
		filterSegment = NewFilterSegment(filter)
	}
	return &SSTableWriter{
		storage:           storage,
		blockManager:      blockManager,
		config:            cfg.SSTable,
		filePath:          filePath,
		dataBlockBuilder:  NewDataBlockBuilder(cfg.SSTable.DataSegment.RestartInterval, blockManager.BlockSize()),
		indexSegment:      NewIndexSegment(),
		summarySegment:    NewSummarySegment(1),
		filterSegment:     filterSegment,
		merkleTree:        NewMerkleTree(),
		footer:            NewFooter(cfg.SSTable),
		currentBlockIndex: 0,
		recordCount:       0,
		minKeyLength:      ^uint32(0),
		maxKeyLength:      0,
		firstRecord:       true,
	}, nil
}

func (sw *SSTableWriter) AddRecord(record Record) error {
	sw.recordCount++
	if sw.firstRecord {
		sw.minTimestamp = record.Timestamp
		sw.maxTimestamp = record.Timestamp
		sw.minKey = append([]byte(nil), record.Key...)
		sw.maxKey = append([]byte(nil), record.Key...)
		sw.firstRecord = false
	} else {
		if utils.Uint128LT(record.Timestamp, sw.minTimestamp) {
			sw.minTimestamp = record.Timestamp
		}
		if utils.Uint128GT(record.Timestamp, sw.maxTimestamp) {
			sw.maxTimestamp = record.Timestamp
		}
		if bytes.Compare(record.Key, sw.minKey) < 0 {
			sw.minKey = append([]byte(nil), record.Key...)
		}
		if bytes.Compare(record.Key, sw.maxKey) > 0 {
			sw.maxKey = append([]byte(nil), record.Key...)
		}
	}
	keyLen := uint32(len(record.Key))
	if keyLen < sw.minKeyLength {
		sw.minKeyLength = keyLen
	}
	if keyLen > sw.maxKeyLength {
		sw.maxKeyLength = keyLen
	}
	if sw.filterSegment != nil && sw.filterSegment.Filter() != nil {
		sw.filterSegment.Filter().Add(record.Key)
	}
	if !sw.dataBlockBuilder.AddRecord(record) {
		if err := sw.flushDataBlock(); err != nil {
			return err
		}
		if !sw.dataBlockBuilder.AddRecord(record) {
			return errors.New("record too large for block")
		}
	}
	return nil
}

func (sw *SSTableWriter) flushDataBlock() error {
	if sw.dataBlockBuilder.RecordCount() == 0 {
		return nil
	}
	blockData, err := sw.dataBlockBuilder.Finish(sw.config.DataSegment.BlockSize)
	if err != nil {
		return err
	}
	_, _, err = sw.storage.WriteSegment(config.SegmentData, blockData)
	if err != nil {
		return err
	}
	blockHash := HashDataBlock(blockData)
	sw.merkleTree.AddLeaf(blockHash)
	indexBlock := NewIndexBlock()
	indexBlock.AddFromDataBlock(sw.dataBlockBuilder, sw.currentBlockIndex)
	sw.indexSegment.AddBlock(indexBlock)
	sw.dataBlockBuilder.Reset()
	sw.currentBlockIndex++
	return nil
}

func (w *SSTableWriter) Finalize() error {
	if w.dataBlockBuilder.RecordCount() > 0 {
		if err := w.flushDataBlock(); err != nil {
			return err
		}
	}
	if err := w.merkleTree.Build(); err != nil {
		return err
	}
	w.footer.NumDataBlocks = w.currentBlockIndex
	w.footer.TotalRecords = w.recordCount
	w.footer.MinTimeStamp = w.minTimestamp
	w.footer.MaxTimeStamp = w.maxTimestamp
	w.footer.MinKeyLength = w.minKeyLength
	w.footer.MaxKeyLength = w.maxKeyLength
	if w.filterSegment != nil {
		filterData, err := w.filterSegment.Encode()
		if err != nil {
			return err
		}
		filterOffset, filterSize, err := w.storage.WriteSegment(config.SegmentFilter, filterData)
		if err != nil {
			return err
		}
		w.footer.FilterHandler = SegmentHandler{
			Offset: filterOffset,
			Size:   filterSize,
		}
	}
	var indexOffsets []uint64
	totalIndexSize := uint64(0)
	if len(w.indexSegment.Blocks) > 0 {
		indexOffsets = make([]uint64, len(w.indexSegment.Blocks))
		for i, indexBlock := range w.indexSegment.Blocks {
			blockData := indexBlock.EncodeIndexBlock()
			offset, size, err := w.storage.WriteSegment(config.SegmentIndex, blockData)
			if err != nil {
				return err
			}
			indexOffsets[i] = offset
			if i == 0 {
				w.footer.IndexHandler.Offset = offset
			}
			totalIndexSize += uint64(size)
		}
		w.footer.IndexHandler.Size = uint32(totalIndexSize)
	} else {
		w.footer.IndexHandler.Offset = 0
		w.footer.IndexHandler.Size = 0
		indexOffsets = []uint64{}
	}
	if len(indexOffsets) > 0 {
		w.summarySegment = BuildSummaryFromIndex(indexOffsets, w.indexSegment.Blocks, 1)
		summaryData := w.summarySegment.Encode()
		summaryOffset, summarySize, err := w.storage.WriteSegment(config.SegmentSummary, summaryData)
		if err != nil {
			return err
		}
		w.footer.SummaryHandler = SegmentHandler{
			Offset: summaryOffset,
			Size:   summarySize,
		}
	} else {
		w.summarySegment = NewSummarySegment(1)
		w.footer.SummaryHandler = SegmentHandler{
			Offset: 0,
			Size:   0,
		}
	}
	metadataData := w.merkleTree.Encode()
	metadataOffset, metadataSize, err := w.storage.WriteSegment(config.SegmentMetadata, metadataData)
	if err != nil {
		return err
	}
	w.footer.MetaDataHandler = SegmentHandler{
		Offset: metadataOffset,
		Size:   metadataSize,
	}
	if err := w.footer.WriteToStorage(w.storage); err != nil {
		return err
	}
	if err := w.storage.Sync(); err != nil {
		return err
	}
	return w.storage.Close()
}

func (w *SSTableWriter) Close() error {
	return w.storage.Close()
}
