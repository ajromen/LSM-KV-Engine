package sstable

import (
	"bytes"
	"fmt"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/encoders"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
	"github.com/ajromen/LSM-KV-Engine/internal/probabilistics"
	"github.com/ajromen/LSM-KV-Engine/internal/shared"
)

const SSTableFileExtension = ".sst"

// SSTableWriter allows writing records into sstable
type SSTableWriter struct {
	storage              SegmentStorage      // low-level writing of segments
	blockManager         *block.BlockManager // writing and encoding data blocks
	filePath             string              // file path where sstable is written (base path if multi file format)
	dataBlockBuilder     *DataBlockBuilder   // data block builder for building data block from records being written into sstable
	indexSegment         *IndexSegment       // index segment of sstable : references data blocks
	ttlIndexSegment      *TTLIndexSegment    // ttl index segment of sstable : contains keys and their ttl
	rangeDelIndexSegment *RangeDelIndexSegment
	currentIndexBlock    *IndexBlock
	valueEncoder         *encoders.AdaptiveEncoder
	summarySegment       *SummarySegment // summary segment of sstable : references index blocks
	filterSegment        *FilterSegment  // filter segment of sstable
	merkleTree           *MerkleTree     // merkle tree of sstable
	metadataSegment      *Metadata       // meta data of sstable
	footer               *Footer         // footer of sstable
	currentBlockIndex    uint32          // tracks current data block index
	recordCount          uint64          // tracks number of records
	minSeqId             uint64          // min seqId (newest record)
	maxSeqId             uint64          // max seqId (newest record)
	minKeyLength         uint32          // smallest key by length
	maxKeyLength         uint32          // largest key by length
	minKey               []byte          // smallest key in sorting order
	maxKey               []byte          // largest ket in sorting order
	firstRecord          bool            // whether it is first record
	Layer                int             // number of the lsm layer
}

func NewSSTableWriter(filePath string, blockManager *block.BlockManager, expectedElements uint64, layer int) (*SSTableWriter, error) {
	cfg := config.GetSettings()
	storage, err := CreateStorage(filePath, blockManager)
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
	valueEncoder := encoders.NewAdaptiveDictEncoderFrequency(2, 5)
	return &SSTableWriter{
		storage:              storage,
		blockManager:         blockManager,
		filePath:             filePath,
		valueEncoder:         valueEncoder,
		dataBlockBuilder:     NewDataBlockBuilder(1, cfg.SSTable.DataSegment.RestartInterval, blockManager.BlockSize(), valueEncoder),
		indexSegment:         NewIndexSegment(uint64(blockManager.BlockSize())),
		ttlIndexSegment:      NewTTLIndexSegment(uint64(blockManager.BlockSize())),
		rangeDelIndexSegment: NewRangeDelIndexSegment(uint64(blockManager.BlockSize())),
		currentIndexBlock:    NewIndexBlock(),
		summarySegment:       NewSummarySegment(1),
		filterSegment:        filterSegment,
		merkleTree:           NewMerkleTree(),
		footer:               NewFooter(),
		currentBlockIndex:    0,
		recordCount:          0,
		minKeyLength:         ^uint32(0),
		maxKeyLength:         0,
		firstRecord:          true,
		Layer:                layer,
	}, nil
}

// AddRecord INSERTS A NEW RECORD INTO DataBlockBuilder OF GIVEN SSTableWriter IN GIVEN ORDER:
// 1. Update min/max key seqId metadata
// 2. Update min/max keylength metadata
// 3. Add key to bloom filter
// 4. Add to TTL index
// 5. Add record to current data block -> block full? -> flush it
func (sw *SSTableWriter) AddRecord(record Record) error {
	sw.recordCount++
	// step 1
	if sw.firstRecord {
		sw.minSeqId = record.SeqId
		sw.maxSeqId = record.SeqId
		sw.minKey = append([]byte(nil), record.Key...)
		sw.maxKey = append([]byte(nil), record.Key...)
		sw.firstRecord = false
	} else {
		if record.SeqId < sw.minSeqId {
			sw.minSeqId = record.SeqId
		}
		if record.SeqId > sw.maxSeqId {
			sw.maxSeqId = record.SeqId
		}
		if bytes.Compare(record.Key, sw.minKey) < 0 {
			sw.minKey = append([]byte(nil), record.Key...)
		}
		if bytes.Compare(record.Key, sw.maxKey) > 0 {
			sw.maxKey = append([]byte(nil), record.Key...)
		}
	}

	// step 2
	keyLen := uint32(len(record.Key))
	if keyLen < sw.minKeyLength {
		sw.minKeyLength = keyLen
	}
	if keyLen > sw.maxKeyLength {
		sw.maxKeyLength = keyLen
	}

	// step 3
	if sw.filterSegment != nil && sw.filterSegment.Filter() != nil {
		sw.filterSegment.Filter().Add(record.Key)
	}

	// step 4
	if record.ExpiresAt != 0 {
		ttlEntry := shared.TTLEntry{
			Key:       record.Key,
			ExpiresAt: record.ExpiresAt,
		}
		sw.ttlIndexSegment.AddEntryToBlock(ttlEntry, config.GetSettings().SSTable.DataSegment.BlockSize)
	}

	// step 5
	if !sw.dataBlockBuilder.AddRecord(record) {
		if err := sw.flushDataBlock(); err != nil {
			return err
		}
		if !sw.dataBlockBuilder.AddRecord(record) {
			sw.recordCount--
			return sw.addRecordChunked(record)
		}
	}
	return nil
}

// addRecordChunked splits a record whose value does not fit in a single block.
// It writes a FIRST chunk, zero or more MIDDLE chunks, and a LAST chunk.
func (sw *SSTableWriter) addRecordChunked(record Record) error {
	blockSize := sw.blockManager.BlockSize()
	headerOverhead := 1 + 10 + 8 + 1 + len(record.Key)*2 + 10 + 60
	if len(sw.dataBlockBuilder.data)+headerOverhead >= blockSize {
		if err := sw.flushDataBlock(); err != nil {
			return err
		}
	}
	available := blockSize - len(sw.dataBlockBuilder.data) - headerOverhead
	if available <= 0 {
		return fmt.Errorf("key too large for data block (key size: %d)", len(record.Key))
	}
	remaining := record.Value
	var firstChunk []byte
	if available >= len(remaining) {
		if !sw.dataBlockBuilder.AddRecord(record) {
			return fmt.Errorf("unexpected: record still doesn't fit")
		}
		return nil
	}
	firstChunk = remaining[:available]
	remaining = remaining[available:]
	if !sw.dataBlockBuilder.AddFirstChunk(record, firstChunk) {
		return fmt.Errorf("cannot write first chunk")
	}
	for len(remaining) > 0 {
		contOverhead := 1 + 10 + 60
		available = blockSize - len(sw.dataBlockBuilder.data) - contOverhead
		if available <= 0 {
			if err := sw.flushDataBlock(); err != nil {
				return err
			}
			available = blockSize - contOverhead
		}
		isLast := len(remaining) <= available
		var chunkData []byte
		if isLast {
			chunkData = remaining
			remaining = nil
		} else {
			chunkData = remaining[:available]
			remaining = remaining[available:]
		}
		chunkType := ChunkTypeMiddle
		if isLast {
			chunkType = ChunkTypeLast
		}
		if !sw.dataBlockBuilder.AddContinuationChunk(chunkType, chunkData) {
			if err := sw.flushDataBlock(); err != nil {
				return err
			}
			if !sw.dataBlockBuilder.AddContinuationChunk(chunkType, chunkData) {
				return fmt.Errorf("cannot write continuation chunk after flush")
			}
		}
	}
	return nil
}

func (sw *SSTableWriter) AddToRangeDel(record Record) error {
	if true {
		rangeDelEntry := shared.RangeDelEntry{
			StartKey: record.Key,
			EndKey:   record.Value,
			SeqId:    record.SeqId,
		}
		sw.rangeDelIndexSegment.AddEntryToBlock(rangeDelEntry, config.GetSettings().SSTable.DataSegment.BlockSize)
	}
	return nil
}

// flushDataBlock FLUSHES DATA BLOCK TO DISK IN GIVEN ORDER:
// 1. Finish the data block
// 2. Write data block to file using segment storage (it uses block manager)
// 3. Hash data block and add it to merkle tree
// 4. Create index block and add it to index segment
// 5. Reset data block builder so next block can be written
func (sw *SSTableWriter) flushDataBlock() error {
	if sw.dataBlockBuilder.RecordCount() == 0 {
		return nil
	}

	// step 1
	blockData, err := sw.dataBlockBuilder.Finish(config.GetSettings().SSTable.DataSegment.BlockSize)
	if err != nil {
		return err
	}

	// step 2
	_, _, err = sw.storage.WriteSegment(enums.SegmentData, blockData)
	if err != nil {
		return err
	}

	// step 3
	blockHash := HashDataBlock(blockData)
	sw.merkleTree.AddLeaf(blockHash)

	// step 4
	sw.currentIndexBlock.AddFromDataBlock(sw.dataBlockBuilder, sw.currentBlockIndex)
	if sw.currentIndexBlock.RealSize >= uint32(config.GetSettings().SSTable.DataSegment.BlockSize) {
		sw.indexSegment.AddBlock(sw.currentIndexBlock)
		sw.currentIndexBlock = NewIndexBlock()
	}

	// step 5
	sw.dataBlockBuilder.Reset()
	sw.currentBlockIndex++
	return nil
}

// Finalize FINALIZES THE SSTABLE IN GIVEN ORDER:
// 1. Flush remaining block if not empty (one block is not flushed because it never got full)
// 2. Build a merkle tree
// 3. Fill metadata
// next steps all update footer segment handler metadata -->>
// 4. Write filter segment on disk
// 5. Write index blocks on disk
// 6. Write summary segment on disk
// 7. Write TTL index on disk
// 8. Write RangeDel index on disk
// 9. Write metadata (merkle tree) segment on disk
// 10. Write footer and sync storage
func (sw *SSTableWriter) Finalize() error {
	// add remaining unfinished index block
	// step 1
	if sw.dataBlockBuilder.RecordCount() > 0 {
		if err := sw.flushDataBlock(); err != nil {
			return err
		}
	}
	if len(sw.currentIndexBlock.Entries) > 0 {
		sw.indexSegment.AddBlock(sw.currentIndexBlock)
	}

	// step 2
	if err := sw.merkleTree.Build(); err != nil {
		return err
	}

	// step 3
	meta := &Metadata{
		Fields: make(map[MetadataFieldID][]byte),
	}
	meta.SetUint64(FieldNumDataBlocks, uint64(sw.currentBlockIndex))
	meta.SetUint64(FieldTotalRecords, sw.recordCount)
	meta.SetUint64(FieldBlockSize, uint64(sw.blockManager.BlockSize()))
	meta.SetUint64(FieldRestartInterval, uint64(config.GetSettings().SSTable.DataSegment.RestartInterval))
	meta.SetBytes(FieldMinKey, sw.minKey)
	meta.SetBytes(FieldMaxKey, sw.maxKey)
	meta.SetUint64(FieldMinSeqId, sw.minSeqId)
	meta.SetUint64(FieldMaxSeqId, sw.maxSeqId)
	meta.SetUint64(FieldMinKeyLength, uint64(sw.minKeyLength))
	meta.SetUint64(FieldMaxKeyLength, uint64(sw.maxKeyLength))
	meta.SetByte(FieldMergeStrategy, byte(enums.Heap))
	meta.SetByte(FieldCompressionType, byte(config.GetSettings().SSTable.DataSegment.Compression))
	sw.metadataSegment = meta
	sw.footer.Format = config.GetSettings().SSTable.Format

	// step 4
	if sw.filterSegment != nil {
		filterData, err := sw.filterSegment.Encode()
		if err != nil {
			return err
		}
		filterOffset, filterSize, err := sw.storage.WriteSegment(enums.SegmentFilter, filterData)
		if err != nil {
			return err
		}
		sw.footer.FilterHandler = SegmentHandler{
			Offset: filterOffset,
			Size:   filterSize,
		}
	}

	// step 5
	var indexOffsets []uint64
	// step 5 - write index blocks to disk and compute proper footer offsets
	if len(sw.indexSegment.Blocks) > 0 {
		indexOffsets = make([]uint64, len(sw.indexSegment.Blocks))
		var firstOffset uint64
		var lastOffsetPlusSize uint64

		for i, indexBlock := range sw.indexSegment.Blocks {
			blockData := indexBlock.EncodeIndexBlock(uint64(config.GetSettings().SSTable.DataSegment.BlockSize))
			offset, size, err := sw.storage.WriteSegment(enums.SegmentIndex, blockData)
			if err != nil {
				return err
			}
			indexOffsets[i] = offset

			if i == 0 {
				firstOffset = offset
			}
			lastOffsetPlusSize = offset + uint64(size)
		}

		sw.footer.IndexHandler.Offset = firstOffset
		sw.footer.IndexHandler.Size = uint32(lastOffsetPlusSize - firstOffset)
	} else {
		sw.footer.IndexHandler.Offset = 0
		sw.footer.IndexHandler.Size = 0
		indexOffsets = []uint64{}
	}

	// step 6
	if len(indexOffsets) > 0 {
		sw.summarySegment = BuildSummaryFromIndex(indexOffsets, sw.indexSegment.Blocks, 1)
		summaryData := sw.summarySegment.Encode()
		summaryOffset, summarySize, err := sw.storage.WriteSegment(enums.SegmentSummary, summaryData)
		if err != nil {
			return err
		}
		sw.footer.SummaryHandler = SegmentHandler{
			Offset: summaryOffset,
			Size:   summarySize,
		}
	} else {
		sw.summarySegment = NewSummarySegment(1)
		sw.footer.SummaryHandler = SegmentHandler{
			Offset: 0,
			Size:   0,
		}
	}

	// step 7
	var ttlOffsets []uint64
	if len(sw.ttlIndexSegment.Blocks) > 0 {
		ttlOffsets = make([]uint64, len(sw.ttlIndexSegment.Blocks))
		var firstOffset uint64
		var lastOffsetPlusSize uint64

		for i, ttlBlock := range sw.ttlIndexSegment.Blocks {
			blockData := ttlBlock.Encode(uint64(config.GetSettings().SSTable.DataSegment.BlockSize))
			offset, size, err := sw.storage.WriteSegment(enums.SegmentTTLIndex, blockData)
			if err != nil {
				return err
			}
			ttlOffsets[i] = offset

			if i == 0 {
				firstOffset = offset
			}
			lastOffsetPlusSize = offset + uint64(size)
		}

		sw.footer.TTLIndexHandler.Offset = firstOffset
		sw.footer.TTLIndexHandler.Size = uint32(lastOffsetPlusSize - firstOffset)
	} else {
		sw.footer.TTLIndexHandler.Offset = 0
		sw.footer.TTLIndexHandler.Size = 0
		ttlOffsets = []uint64{}
	}

	// step 8
	var rangeDelOffsets []uint64
	if len(sw.rangeDelIndexSegment.Blocks) > 0 {
		rangeDelOffsets = make([]uint64, len(sw.rangeDelIndexSegment.Blocks))
		var firstOffset uint64
		var lastOffsetPlusSize uint64
		for i, rangeDelBlock := range sw.rangeDelIndexSegment.Blocks {
			blockData := rangeDelBlock.Encode(uint64(config.GetSettings().SSTable.DataSegment.BlockSize))
			offset, size, err := sw.storage.WriteSegment(enums.SegmentRangeDelIndex, blockData)
			if err != nil {
				return err
			}
			rangeDelOffsets[i] = offset
			if i == 0 {
				firstOffset = offset
			}
			lastOffsetPlusSize = offset + uint64(size)
		}
		sw.footer.RangeDelIndexHandler.Offset = firstOffset
		sw.footer.RangeDelIndexHandler.Size = uint32(lastOffsetPlusSize - firstOffset)
	} else {
		sw.footer.RangeDelIndexHandler.Offset = 0
		sw.footer.RangeDelIndexHandler.Size = 0
		rangeDelOffsets = []uint64{}
	}

	// step 9
	merkleTreeData := sw.merkleTree.Encode()
	merkleTreeOffset, merkleTreeSize, err := sw.storage.WriteSegment(enums.SegmentMerkleTree, merkleTreeData)
	if err != nil {
		return err
	}
	sw.footer.MerkleHandler = SegmentHandler{
		Offset: merkleTreeOffset,
		Size:   merkleTreeSize,
	}
	metaDataData := sw.metadataSegment.Encode()
	metaDataOffset, metaDataSize, err := sw.storage.WriteSegment(enums.SegmentMetadata, metaDataData)
	if err != nil {
		return err
	}
	sw.footer.MetaDataHandler = SegmentHandler{
		Offset: metaDataOffset,
		Size:   metaDataSize,
	}

	// step 10
	dictData := sw.valueEncoder.SaveDict()
	dictOffset, dictSize, err := sw.storage.WriteSegment(enums.SegmentDictionary, dictData)
	if err != nil {
		return err
	}
	sw.footer.DictionaryHandler = SegmentHandler{
		Offset: dictOffset,
		Size:   dictSize,
	}
	if err := sw.footer.WriteToStorage(sw.storage); err != nil {
		return err
	}
	if err := sw.storage.Sync(); err != nil {
		return err
	}
	return sw.storage.Close()
}

func (sw *SSTableWriter) Close() error {
	return sw.storage.Close()
}
