package sstable

import (
	"bytes"
	"errors"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/probabilistics"
	"github.com/ajromen/LSM-KV-Engine/internal/utils"
)

const SSTableFileExtension = ".sst"

// SSTableWriter allows writing records into sstable
type SSTableWriter struct {
	storage           SegmentStorage       // low-level writing of segments
	blockManager      *block.BlockManager  // writing and encoding data blocks
	config            config.SSTableConfig // system SSTable configuration
	filePath          string               // file path where sstable is written (base path if multi file format)
	dataBlockBuilder  *DataBlockBuilder    // data block builder for building data block from records being written into sstable
	indexSegment      *IndexSegment        // index segment of sstable : references data blocks
	currentIndexBlock *IndexBlock
	summarySegment    *SummarySegment // summary segment of sstable : references index blocks
	filterSegment     *FilterSegment  // filter segment of sstable
	merkleTree        *MerkleTree     // merkle tree of sstable
	footer            *Footer         // footer of sstable
	currentBlockIndex uint32          // tracks current data block index
	recordCount       uint64          // tracks number of records
	minTimestamp      utils.Uint128   // min timestamp (newest record)
	maxTimestamp      utils.Uint128   // max timestamp (oldest record)
	minKeyLength      uint32          // smallest key by length
	maxKeyLength      uint32          // largest key by length
	minKey            []byte          // smallest key in sorting order
	maxKey            []byte          // largest ket in sorting order
	firstRecord       bool            // whether it is first record
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
		dataBlockBuilder:  NewDataBlockBuilder(1, cfg.SSTable.DataSegment.RestartInterval, blockManager.BlockSize()),
		indexSegment:      NewIndexSegment(uint64(blockManager.BlockSize())),
		currentIndexBlock: NewIndexBlock(),
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

// AddRecord INSERTS A NEW RECORD INTO DataBlockBuilder OF GIVEN SSTableWriter IN GIVEN ORDER:
// 1. Update min/max key timestamp metadata
// 2. Update min/max keylength metadata
// 3. Add key to bloom filter
// 4. Add record to current data block -> block full? -> flush it
func (sw *SSTableWriter) AddRecord(record Record) error {
	sw.recordCount++
	// step 1
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
	blockData, err := sw.dataBlockBuilder.Finish(sw.config.DataSegment.BlockSize)
	if err != nil {
		return err
	}

	// step 2
	_, _, err = sw.storage.WriteSegment(config.SegmentData, blockData)
	if err != nil {
		return err
	}

	// step 3
	blockHash := HashDataBlock(blockData)
	sw.merkleTree.AddLeaf(blockHash)

	// step 4
	sw.currentIndexBlock.AddFromDataBlock(sw.dataBlockBuilder, sw.currentBlockIndex)
	if sw.currentIndexBlock.RealSize >= uint32(sw.config.IndexSegment.IndexBlockSize) {
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
// 3. Fill footer metadata
// next steps all update footer segment handler metadata -->>
// 4. Write filter segment on disk
// 5. Write index blocks on disk
// 6. Write summary segment on disk
// 7. Write metadata (merkle tree) segment on disk
// 8. Write footer and sync storage
func (w *SSTableWriter) Finalize() error {
	// add remaining unfinished index block
	// step 1
	if w.dataBlockBuilder.RecordCount() > 0 {
		if err := w.flushDataBlock(); err != nil {
			return err
		}
	}
	if len(w.currentIndexBlock.Entries) > 0 {
		w.indexSegment.AddBlock(w.currentIndexBlock)
	}

	// step 2
	if err := w.merkleTree.Build(); err != nil {
		return err
	}

	// step 3
	w.footer.NumDataBlocks = w.currentBlockIndex
	w.footer.TotalRecords = w.recordCount
	w.footer.MinTimeStamp = w.minTimestamp
	w.footer.MaxTimeStamp = w.maxTimestamp
	w.footer.MinKeyLength = w.minKeyLength
	w.footer.MaxKeyLength = w.maxKeyLength
	w.footer.Format = w.config.Format

	// step 4
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

	// step 5
	var indexOffsets []uint64
	// step 5 - write index blocks to disk and compute proper footer offsets
	if len(w.indexSegment.Blocks) > 0 {
		indexOffsets = make([]uint64, len(w.indexSegment.Blocks))
		var firstOffset uint64
		var lastOffsetPlusSize uint64

		for i, indexBlock := range w.indexSegment.Blocks {
			blockData := indexBlock.EncodeIndexBlock(config.DefaultIndexBlockSize)
			offset, size, err := w.storage.WriteSegment(config.SegmentIndex, blockData)
			if err != nil {
				return err
			}
			indexOffsets[i] = offset

			if i == 0 {
				firstOffset = offset
			}
			lastOffsetPlusSize = offset + uint64(size)
		}

		w.footer.IndexHandler.Offset = firstOffset
		w.footer.IndexHandler.Size = uint32(lastOffsetPlusSize - firstOffset)
	} else {
		w.footer.IndexHandler.Offset = 0
		w.footer.IndexHandler.Size = 0
		indexOffsets = []uint64{}
	}

	// step 6
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

	// step 7
	metadataData := w.merkleTree.Encode()
	metadataOffset, metadataSize, err := w.storage.WriteSegment(config.SegmentMetadata, metadataData)
	if err != nil {
		return err
	}
	w.footer.MetaDataHandler = SegmentHandler{
		Offset: metadataOffset,
		Size:   metadataSize,
	}

	// step 8
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
