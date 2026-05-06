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
	"github.com/ajromen/LSM-KV-Engine/internal/shared"
	"github.com/ajromen/LSM-KV-Engine/internal/utils"
)

// SSTableReader allows reading an SSTable file, accesing singular records and validating data integrity
type SSTableReader struct {
	filePath            string              // file path of given sstable file (base path if multi file format)
	Id                  int                 // SSTable segment id
	Layer               int                 // number of the lsm layer
	SizeBytes           int64               //file size in bytes
	storage             SegmentStorage      // low-level reading of segments
	blockManager        *block.BlockManager // reading and decoding data blocks
	footer              *Footer             // footer of given sstable
	Metadata            *Metadata
	SummarySegment      *SummarySegment  // summary segment (read into RAM)
	filterSegment       *FilterSegment   // filter segment (read into RAM)
	merkleTree          *MerkleTree      // merkle tree - metadata segment (read into RAM)
	TTLIndexSegment     *TTLIndexSegment // read once
	valueDecoder        *encoders.AdaptiveEncoder
	fragmentedRangeDels []shared.RangeDelEntry
	rangeDelLoaded      bool
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
		storage:         storage,
		blockManager:    blockManager,
		filePath:        filePath,
		footer:          footer,
		Layer:           layer,
		Metadata:        meta,
		Id:              id,
		SizeBytes:       fileSize,
		TTLIndexSegment: NewTTLIndexSegment(blockSize),
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
	if err := w.storage.Sync(); err != nil {
		return nil, fmt.Errorf("failed to sync storage: %w", err)
	}
	if err := w.storage.Close(); err != nil {
		return nil, fmt.Errorf("failed to close writer storage: %w", err)
	}

	format := config.GetSettings().SSTable.Format
	var newStorage SegmentStorage
	var err error
	switch format {
	case enums.FormatSingleFile:
		newStorage, err = OpenSingleFileStorage(w.filePath)
	case enums.FormatMultiFile:
		newStorage, err = OpenMultiFileStorage(w.filePath)
	default:
		return nil, fmt.Errorf("unsupported format")
	}
	if err != nil {
		return nil, err
	}
	newStorage.SetBlockManager(w.blockManager)

	r := &SSTableReader{
		storage:         newStorage,
		Layer:           w.Layer,
		filePath:        w.filePath,
		blockManager:    w.blockManager,
		merkleTree:      w.merkleTree,
		filterSegment:   w.filterSegment,
		footer:          w.footer,
		SummarySegment:  w.summarySegment,
		valueDecoder:    w.valueEncoder,
		Metadata:        w.metadataSegment,
		Id:              id,
		TTLIndexSegment: w.ttlIndexSegment,
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

func (r *SSTableReader) GetTTLEntries() ([]shared.TTLEntry, error) {
	entries := make([]shared.TTLEntry, 0)
	if r.footer.TTLIndexHandler.Size == 0 {
		return entries, nil
	}
	blockSize := uint64(r.blockManager.BlockSize())
	baseOffset := r.footer.TTLIndexHandler.Offset
	totalSize := uint64(r.footer.TTLIndexHandler.Size)

	for off := uint64(0); off < totalSize; off += blockSize {
		buf, err := r.storage.ReadSegment(enums.SegmentTTLIndex, baseOffset+off, uint32(blockSize))
		if err != nil {
			return nil, err
		}
		b, err := DecodeTTLIndexBlock(buf)
		if err != nil {
			return nil, err
		}
		entries = append(entries, b.Entries...)
	}
	return entries, nil
}

func (r *SSTableReader) GetRangeDelEntries() ([]shared.RangeDelEntry, error) {
	entries := make([]shared.RangeDelEntry, 0)
	if r.footer.RangeDelIndexHandler.Size == 0 {
		return entries, nil
	}
	blockSize := uint64(r.blockManager.BlockSize())
	baseOffset := r.footer.RangeDelIndexHandler.Offset
	totalSize := uint64(r.footer.RangeDelIndexHandler.Size)
	for off := uint64(0); off < totalSize; off += blockSize {
		buf, err := r.storage.ReadSegment(enums.SegmentRangeDelIndex, baseOffset+off, uint32(blockSize))
		if err != nil {
			return nil, err
		}
		b, err := DecodeRangeDelIndexBlock(buf)
		if err != nil {
			return nil, err
		}
		entries = append(entries, b.Entries...)
	}
	return entries, nil
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

	indexBlockSize := r.blockManager.BlockSize()

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

func (r *SSTableReader) LoadRangeDels() error {
	if r.rangeDelLoaded {
		return nil
	}
	r.rangeDelLoaded = true
	if r.footer.RangeDelIndexHandler.Size == 0 {
		return nil
	}
	blockSize := uint64(r.blockManager.BlockSize())
	totalSize := uint64(r.footer.RangeDelIndexHandler.Size)
	baseOffset := r.footer.RangeDelIndexHandler.Offset
	var all []shared.RangeDelEntry
	for off := uint64(0); off < totalSize; off += blockSize {
		buf, err := r.storage.ReadSegment(enums.SegmentRangeDelIndex, baseOffset+off, uint32(blockSize))
		if err != nil {
			return err
		}
		blk, err := DecodeRangeDelIndexBlock(buf)
		if err != nil {
			return err
		}
		all = append(all, blk.Entries...)
	}
	r.fragmentedRangeDels = utils.FragmentRangeTombstones(all)
	return nil
}

func (r *SSTableReader) IsCoveredByRangeDel(key []byte, keySeqId uint64) bool {
	return utils.IsCoveredByRangeTombstone(r.fragmentedRangeDels, key, keySeqId)
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
			if config.GetSettings().Debug {
				fmt.Printf("Filter of sstable informs that no key is present in that sstable.\n")
			}
			return nil, nil
		}
	}

	// check whether the key is in given range of sstable keys
	minKey := r.Metadata.GetBytes(FieldMinKey)
	maxKey := r.Metadata.GetBytes(FieldMaxKey)
	if bytes.Compare(minKey, key) > 0 || bytes.Compare(maxKey, key) < 0 {
		if config.GetSettings().Debug {
			fmt.Printf("Skipping sstable cause it doesn't contain keys in given range\n")
		}
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
	iter, err := NewSSTableIteratorRaw(r)
	if err != nil {
		return nil, err
	}
	iter.Seek(Record{Key: key})
	if !iter.Valid() {
		return nil, nil
	}
	rec := iter.Key()
	if !bytes.Equal(rec.Key, key) {
		return nil, nil
	}

	// check whether it is deleted by range delete
	rangeEntries, err := r.GetRangeDelEntries()
	if err != nil {
		return nil, err
	}
	r.fragmentedRangeDels = utils.FragmentRangeTombstones(rangeEntries)
	if r.IsCoveredByRangeDel(key, rec.SeqId) {
		return nil, nil
	}

	return &Record{
		SeqId:     rec.SeqId,
		OpType:    rec.OpType,
		Key:       rec.Key,
		Value:     rec.Value,
		ExpiresAt: rec.ExpiresAt,
	}, nil
}

func (reader *SSTableReader) OverlapsRange(start, end []byte) bool {
	if reader == nil || reader.SummarySegment == nil {
		return true
	}

	minKey := reader.Metadata.GetBytes(FieldMinKey)
	maxKey := reader.Metadata.GetBytes(FieldMaxKey)

	if len(minKey) == 0 || len(maxKey) == 0 {
		return true
	}

	if len(end) > 0 && bytes.Compare(minKey, end) > 0 {
		return false
	}
	if len(start) > 0 && bytes.Compare(maxKey, start) < 0 {
		return false
	}

	return true
}

// Validate reads every data block, hashes it, and checks the result against
// the stored Merkle tree. Returns the ValidationResult or an error.
func (r *SSTableReader) Validate() (*ValidationResult, error) {
	if err := r.loadMerkleTree(); err != nil {
		return nil, fmt.Errorf("load merkle tree: %w", err)
	}
	if r.merkleTree == nil {
		return nil, errors.New("no merkle tree in this SSTable")
	}

	numBlocks := int(r.merkleTree.NumDataBlocks)
	blockSize := uint64(r.blockManager.BlockSize())
	blockHashes := make([][32]byte, numBlocks)

	for i := 0; i < numBlocks; i++ {
		offset := uint64(i) * blockSize
		data, err := r.storage.ReadSegment(enums.SegmentData, offset, uint32(blockSize))
		if err != nil {
			return nil, fmt.Errorf("read data block %d: %w", i, err)
		}
		blockHashes[i] = HashDataBlock(data)
	}

	return r.merkleTree.Verify(blockHashes)
}
