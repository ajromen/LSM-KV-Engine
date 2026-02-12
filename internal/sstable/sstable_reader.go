package sstable

import (
	"crypto/sha256"
	"fmt"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/utils"
)

type SSTableReader struct {
	filePath          string
	config            *config.Config
	storage           SegmentStorage
	blockManager      *block.BlockManager
	footer            *Footer
	filterSegment     *FilterSegment
	summarySegment    *SummarySegment
	indexBlockOffsets []uint64
	indexBlockSizes   []int
	indexBlockCache   map[int]*IndexBlock
	merkleTree        *MerkleTree
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
	var summarySegment *SummarySegment
	if footer.SummaryHandler.Size > 0 {
		summaryData, err := storage.ReadSegment(
			config.SegmentSummary,
			footer.SummaryHandler.Offset,
			footer.SummaryHandler.Size,
		)
		if err != nil {
			storage.Close()
			return nil, fmt.Errorf("failed to read summary: %w", err)
		}

		summarySegment, err = DecodeSummarySegment(summaryData)
		if err != nil {
			storage.Close()
			return nil, fmt.Errorf("failed to decode summary: %w", err)
		}
	}
	var filterSegment *FilterSegment
	if footer.FilterHandler.Size > 0 {
		filterData, err := storage.ReadSegment(
			config.SegmentFilter,
			footer.FilterHandler.Offset,
			footer.FilterHandler.Size,
		)
		if err != nil {
			storage.Close()
			return nil, fmt.Errorf("failed to read filter: %w", err)
		}

		filterSegment, err = DecodeFilterSegment(filterData)
		if err != nil {
			storage.Close()
			return nil, fmt.Errorf("failed to decode filter: %w", err)
		}
	}
	var merkleTree *MerkleTree
	if footer.MetaDataHandler.Size > 0 {
		merkleData, err := storage.ReadSegment(
			config.SegmentMetadata,
			footer.MetaDataHandler.Offset,
			footer.MetaDataHandler.Size,
		)
		if err != nil {
			storage.Close()
			return nil, fmt.Errorf("failed to read merkle tree: %w", err)
		}

		merkleTree, err = DecodeMerkleTree(merkleData)
		if err != nil {
			storage.Close()
			return nil, fmt.Errorf("failed to decode merkle tree: %w", err)
		}
	}
	var indexBlockOffsets []uint64
	var indexBlockSizes []int
	if summarySegment != nil && len(summarySegment.Entries) > 0 {
		indexBlockOffsets = make([]uint64, len(summarySegment.Entries))
		for i, entry := range summarySegment.Entries {
			indexBlockOffsets[i] = entry.IndexBlockOffset
		}
		indexBlockSizes = make([]int, len(summarySegment.Entries))
		for i := 0; i < len(summarySegment.Entries)-1; i++ {
			indexBlockSizes[i] = int(summarySegment.Entries[i+1].IndexBlockOffset - summarySegment.Entries[i].IndexBlockOffset)
		}
		if len(summarySegment.Entries) > 0 {
			lastOffset := summarySegment.Entries[len(summarySegment.Entries)-1].IndexBlockOffset
			indexBlockSizes[len(summarySegment.Entries)-1] = int(footer.IndexHandler.Size) - int(lastOffset)
		}
	}
	return &SSTableReader{
		filePath:          filePath,
		config:            cnfig,
		storage:           storage,
		blockManager:      blockManager,
		footer:            footer,
		filterSegment:     filterSegment,
		summarySegment:    summarySegment,
		indexBlockOffsets: indexBlockOffsets,
		indexBlockSizes:   indexBlockSizes,
		indexBlockCache:   make(map[int]*IndexBlock),
		merkleTree:        merkleTree,
	}, nil
}

func (sr *SSTableReader) loadIndexBlock(blockNum int) (*IndexBlock, error) {
	if cached, ok := sr.indexBlockCache[blockNum]; ok {
		return cached, nil
	}
	if blockNum < 0 || blockNum >= len(sr.indexBlockOffsets) {
		return nil, fmt.Errorf("index block %d out of range", blockNum)
	}
	offset := sr.footer.IndexHandler.Offset + sr.indexBlockOffsets[blockNum]
	size := uint32(sr.indexBlockSizes[blockNum])
	indexBlockData, err := sr.storage.ReadSegment(config.SegmentIndex, offset, size)
	if err != nil {
		return nil, fmt.Errorf("failed to read index block %d: %w", blockNum, err)
	}
	indexBlock, err := DecodeIndexBlock(indexBlockData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode index block %d: %w", blockNum, err)
	}
	sr.indexBlockCache[blockNum] = indexBlock
	return indexBlock, nil
}

func (sr *SSTableReader) Get(key []byte) (*Record, bool, error) {
	if sr.filterSegment != nil && sr.filterSegment.Filter() != nil {
		if !sr.filterSegment.Filter().MightContain(key) {
			return nil, false, nil
		}
	}
	var startBlockNum, endBlockNum int
	if sr.summarySegment != nil {
		blockNum := sr.summarySegment.FindIndexBlockNumber(key)
		if blockNum < 0 {
			return nil, false, nil
		}

		startBlockNum = blockNum
		endBlockNum = blockNum + int(sr.summarySegment.SamplingDegree) - 1
		if endBlockNum >= int(sr.summarySegment.TotalIndexBlocks) {
			endBlockNum = int(sr.summarySegment.TotalIndexBlocks) - 1
		}
	} else {
		return sr.linearScanGet(key)
	}
	for blockNum := startBlockNum; blockNum <= endBlockNum; blockNum++ {
		indexBlock, err := sr.loadIndexBlock(blockNum)
		if err != nil {
			return nil, false, err
		}
		entryIdx := indexBlock.FindBlock(key)
		if entryIdx >= 0 {
			logicalBlockIndex := indexBlock.Entries[entryIdx].BlockIndex
			return sr.getFromBlock(logicalBlockIndex, key)
		}
	}
	return nil, false, nil
}

func (sr *SSTableReader) linearScanGet(key []byte) (*Record, bool, error) {
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

func (sr *SSTableReader) ValidateMerkleTree() (*ValidationResult, error) {
	if sr.merkleTree == nil {
		return &ValidationResult{
			Valid: false,
		}, nil
	}
	blockHashes := make([][32]byte, 0, sr.footer.NumDataBlocks)
	for blockIdx := uint32(0); blockIdx < sr.footer.NumDataBlocks; blockIdx++ {
		blockKey := block.BlockKey{
			FilePath: sr.filePath,
			Offset:   blockIdx,
		}
		blockData, err := sr.blockManager.Read(blockKey)
		if err != nil {
			return nil, fmt.Errorf("failed to read block %d: %w", blockIdx, err)
		}
		blockHash := sha256.Sum256(blockData)
		blockHashes = append(blockHashes, blockHash)
	}
	return sr.merkleTree.Verify(blockHashes)
}

func (sr *SSTableReader) Close() error {
	return nil
}

func (sr *SSTableReader) GetMerkleProof(blockIndex uint32) (*MerkleProof, error) {
	if sr.merkleTree == nil {
		return nil, fmt.Errorf("no merkle tree metadata found")
	}
	return sr.merkleTree.GetProof(blockIndex)
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
	if err := iter.loadBlock(); err != nil {
		return nil, err
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
