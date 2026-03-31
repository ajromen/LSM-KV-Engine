/*
TODO:
sstable needs to use sstable config
BlockManager should read index blocks too
*/

package sstable

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/encoders"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
)

// SSTableReader allows reading an SSTable file, accesing singular records and validating data integrity
type SSTableReader struct {
	filePath       string              // file path of given sstable file (base path if multi file format)
	Id             int                 // SSTable segment id
	Layer          int                 // number of the lsm layer
	SizeBytes      int64               //file size in bytes
	storage        SegmentStorage      // low-level reading of segments
	blockManager   *block.BlockManager // reading and decoding data blocks
	footer         *Footer             // footer of given sstable
	Metadata       *Metadata
	SummarySegment *SummarySegment // summary segment (read into RAM)
	filterSegment  *FilterSegment  // filter segment (read into RAM)
	merkleTree     *MerkleTree     // merkle tree - metadata segment (read into RAM)
	valueDecoder   *encoders.AdaptiveEncoder
}

type ReaderOptions struct {
	id       int
	filePath string
	format   enums.SSTableFormat
	layer    int
}

// opens an SSTable file and loads all necessary segments into RAM
func NewSSTableReader(id int, filePath string, format enums.SSTableFormat, layer int) (*SSTableReader, error) {
	var storage SegmentStorage
	var err error
	switch format {
	case enums.FormatSingleFile:
		storage, err = OpenSingleFileStorage(filePath)
	case enums.FormatMultiFile:
		storage, err = OpenMultiFileStorage(filePath)
	default:
		return nil, fmt.Errorf("unsupported format")
	}

	if err != nil {
		return nil, err
	}
	offset := uint64(0)
	var fileSize int64
	switch s := storage.(type) {
	case *SingleFileStorage:
		info, err := s.File().Stat()
		if err != nil {
			return nil, fmt.Errorf("single storage failed to stat file: %w", err)
		}
		fileSize = info.Size()
		offset = uint64(info.Size()) - FooterSize
	case *MultiFileStorage:
		offset = 0
		info, err := os.Stat(filePath + string(DataSegmentExtension))
		if err != nil {
			return nil, fmt.Errorf("multi storage failed to stat file: %w", err)
		}
		fileSize = info.Size()
	}
	// read and validate footer
	footerData, err := storage.ReadSegment(enums.SegmentFooter, offset, FooterSize)
	if err != nil {
		return nil, err
	}
	footer := &Footer{}
	footer.Decode(footerData)
	if err := footer.Validate(); err != nil {
		return nil, err
	}
	metaDataData, err := storage.ReadSegment(enums.SegmentMetadata, footer.MetaDataHandler.Offset, footer.MetaDataHandler.Size)
	if err != nil {
		return nil, err
	}
	meta, err := Decode(metaDataData)
	if err != nil {
		return nil, err
	}
	blockSize, _ := meta.GetUint64(FieldBlockSize)
	blockManager := block.NewBlockManager(int(blockSize))
	storage.SetBlockManager(blockManager)
	// initialize reader
	reader := &SSTableReader{
		storage:      storage,
		blockManager: blockManager,
		filePath:     filePath,
		footer:       footer,
		Layer:        layer,
		Metadata:     meta,
		Id:           id,
		SizeBytes:    fileSize,
	}
	// load filter into RAM
	if err := reader.loadFilter(); err != nil {
		reader.filterSegment = nil
	}
	if err := reader.loadDictionary(); err != nil {
		return nil, fmt.Errorf("failed to load dictionary: %w", err)
	}
	return reader, nil
}

// NewSSTableReaderFromWriter make sure writer finalize has been run before
func NewSSTableReaderFromWriter(w *SSTableWriter, id int) (*SSTableReader, error) {
	if err := w.storage.Restart(); err != nil {
		return nil, fmt.Errorf("failed to restart storage: %w", err)
	}

	r := &SSTableReader{
		storage:        w.storage,
		Layer:          w.Layer,
		filePath:       w.filePath,
		blockManager:   w.blockManager,
		merkleTree:     w.merkleTree,
		filterSegment:  w.filterSegment,
		footer:         w.footer,
		SummarySegment: w.summarySegment,
		valueDecoder:   w.valueEncoder, // TODO check encoder is decoder
		Metadata:       w.metadataSegment,
		Id:             id,
	}
	switch s := r.storage.(type) {
	case *SingleFileStorage:
		info, err := s.File().Stat()
		if err != nil {
			return nil, fmt.Errorf("single storage failed to stat file: %w", err)
		}
		r.SizeBytes = info.Size()
	case *MultiFileStorage:
		info, err := os.Stat(r.filePath + string(DataSegmentExtension))
		if err != nil {
			return nil, fmt.Errorf("multi storage failed to stat file: %w", err)
		}
		r.SizeBytes = info.Size()
	}

	return r, nil
}

// loadSummary reads and decodes summary segment, which maps key ranges into index blocks
func (r *SSTableReader) loadSummary() error {
	data, err := r.storage.ReadSegment(enums.SegmentSummary, r.footer.SummaryHandler.Offset, r.footer.SummaryHandler.Size)
	if err != nil {
		return err
	}
	summary, err := DecodeSummarySegment(data)
	if err != nil {
		return err
	}
	r.SummarySegment = summary
	return nil
}

// loadFilter reads and decodes the filter segment -> filter allows quickly checking whether a key exists or nott
func (r *SSTableReader) loadFilter() error {
	if r.footer.FilterHandler.Size == 0 {
		return errors.New("no filter segment")
	}
	data, err := r.storage.ReadSegment(enums.SegmentFilter, r.footer.FilterHandler.Offset, r.footer.FilterHandler.Size)
	if err != nil {
		return err
	}
	filter, err := DecodeFilterSegment(data)
	if err != nil {
		return err
	}
	r.filterSegment = filter
	return nil
}

// loadMerkleTree reads and decodes the Merkle tree for data integrity verification
func (r *SSTableReader) loadMerkleTree() error {
	data, err := r.storage.ReadSegment(enums.SegmentMerkleTree, r.footer.MerkleHandler.Offset, r.footer.MerkleHandler.Size)
	if err != nil {
		return err
	}
	tree := &MerkleTree{}
	tree, err = tree.Decode(data)
	if err != nil {
		return err
	}
	r.merkleTree = tree
	return nil
}

// loadIndexBlock reads a specific index block from index segment -> NEEDS TO BE FIXED!
func (r *SSTableReader) loadIndexBlock(blockNumber int) (*IndexBlock, error) {

	indexBlockSize := config.GetSettings().SSTable.DataSegment.BlockSize

	offset := r.footer.IndexHandler.Offset +
		uint64(blockNumber)*uint64(indexBlockSize)
	data, err := r.storage.ReadSegment(
		enums.SegmentIndex,
		offset,
		uint32(indexBlockSize),
	)
	if err != nil {
		return nil, err
	}

	block, err := DecodeIndexBlock(data)
	if err != nil {
		return nil, err
	}

	return block, nil
}

func (r *SSTableReader) loadDictionary() error {
	if r.footer.DictionaryHandler.Size == 0 {
		r.valueDecoder = encoders.NewAdaptiveDictEncoderFrequency(2, 5)
		return nil
	}
	data, err := r.storage.ReadSegment(
		enums.SegmentDictionary,
		r.footer.DictionaryHandler.Offset,
		r.footer.DictionaryHandler.Size,
	)
	if err != nil {
		return err
	}
	decoder := encoders.NewAdaptiveDictEncoderFrequency(2, 5)
	if err := decoder.ReadDictionary(data); err != nil {
		return err
	}
	r.valueDecoder = decoder
	return nil
}

// Get looks up a record by key in the SSTable in given order:
// 1. Use filter segment to check if key might exist
// 2. Use summary segment to find the correct index block that should contain key
// 3. Load correct index block and locate the data block that should contain key
// 4. Read the data block
// 5. Iterate through the read data block to find the exact record
func (r *SSTableReader) Get(key []byte) (*Record, error) {
	// step 1
	if r.filterSegment != nil && r.filterSegment.Filter() != nil {
		if !r.filterSegment.Filter().MightContain(key) {
			return nil, nil
		}
	}

	// check whether the key is in given range of sstable keys
	minKey := r.Metadata.GetBytes(FieldMinKey)
	maxKey := r.Metadata.GetBytes(FieldMaxKey)
	if bytes.Compare(minKey, key) > 0 || bytes.Compare(maxKey, key) < 0 {
		return nil, nil
	}

	// load merkle tree, dictionary and summary segment into ram
	if err := r.loadSummary(); err != nil {
		return nil, err
	}
	if err := r.loadMerkleTree(); err != nil {
		return nil, err
	}
	if err := r.loadDictionary(); err != nil {
		return nil, err
	}

	// step 2
	indexBlockNum := r.SummarySegment.FindIndexBlockNumber(key)
	if indexBlockNum < 0 {
		return nil, nil
	}

	// step 3.1
	indexBlock, err := r.loadIndexBlock(indexBlockNum)
	if err != nil {
		return nil, err
	}

	// step 3.2
	entryIdx := indexBlock.FindBlock(key)
	if entryIdx < 0 {
		return nil, nil
	}

	// step 4
	dataBlockIdx := indexBlock.Entries[entryIdx].BlockIndex
	dataFilePath := r.filePath
	if _, err := os.Stat(r.filePath + string(DataSegmentExtension)); err == nil {
		dataFilePath = r.filePath + string(DataSegmentExtension)
	}
	blockKey := block.BlockKey{
		FilePath: dataFilePath,
		Offset:   dataBlockIdx,
	}
	blockData, err := r.blockManager.Read(blockKey)
	if err != nil {
		return nil, err
	}
	if validated := r.merkleTree.ValidateBlock(dataBlockIdx, blockData); !validated {
		return nil, errors.New("sstable data corruption detected (merkle root mismatch)")
	}

	// step 5
	restartInterval, _ := r.Metadata.GetUint64(FieldRestartInterval)

	iterator, err := NewDataBlockIteratorRaw(blockData, int(restartInterval), encoders.PrefixCompression, r.valueDecoder)
	if err != nil {
		return nil, err
	}
	iterator.Seek(Record{Key: key})
	if !iterator.Valid() {
		return nil, nil
	}
	rec := iterator.Key()
	if !bytes.Equal(rec.Key, key) {
		return nil, nil
	}
	return &Record{
		SeqId:     rec.SeqId,
		Tombstone: rec.Tombstone,
		Key:       rec.Key,
		Value:     rec.Value,
	}, nil
}
