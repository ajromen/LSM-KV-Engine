/*
TODO:
sstable needs to use sstable config
blockManager should read index blocks too
*/

package sstable

import (
	"bytes"
	"errors"
	"os"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
)

// SSTableReader allows reading an SSTable file, accesing singular records and validating data integrity
type SSTableReader struct {
	storage        SegmentStorage      // low-level reading of segments
	blockManager   *block.BlockManager // reading and decoding data blocks
	filePath       string              // file path of given sstable file (base path if multi file format)
	footer         *Footer             // footer of given sstable
	summarySegment *SummarySegment     // summary segment (read into RAM)
	filterSegment  *FilterSegment      // filter segment (read into RAM)
	merkleTree     *MerkleTree         // merkle tree - metadata segment (read into RAM)
	config         *config.Config      // config for given sstable
}

// detectSSTableFormat determines if an SSTable is single-file or multi-file format
func detectSSTableFormat(filePath string) (byte, error) {
	// Check if multi-file format exists (look for .footer file)
	footerFile := filePath + ".footer"
	if _, err := os.Stat(footerFile); err == nil {
		// .footer file exists, this is multi-file format
		return 1, nil
	}

	// Otherwise it's single-file format
	return 0, nil
}

// NewSSTableReader opens an SSTable file and loads all necessary segments into RAM
func NewSSTableReader(filePath string, cfg *config.Config) (*SSTableReader, error) {
	fileFormat, err := detectSSTableFormat(filePath)
	if err != nil {
		return nil, err
	}
	var storage SegmentStorage
	if fileFormat == 0 {
		// Single-file format
		storage, err = OpenSingleFileStorage(filePath)
	} else {
		// Multi-file format
		storage, err = OpenMultiFileStorage(filePath)
	}
	if err != nil {
		return nil, err
	}
	offset := uint64(0)
	switch s := storage.(type) {
	case *SingleFileStorage:
		info, err := s.File().Stat()
		if err != nil {
			return nil, err
		}
		offset = uint64(info.Size()) - FooterSize
	case *MultiFileStorage:
		offset = 0
	}
	// read and validate footer
	footerData, err := storage.ReadSegment(config.SegmentFooter, offset, FooterSize)
	if err != nil {
		return nil, err
	}
	footer := &Footer{}
	footer.Decode(footerData)
	if err := footer.Validate(); err != nil {
		return nil, err
	}
	blockManager := block.NewBlockManager(int(footer.BlockSize), 100)
	// initialize reader
	reader := &SSTableReader{
		storage:      storage,
		blockManager: blockManager,
		filePath:     filePath,
		footer:       footer,
		config:       cfg,
	}
	// load summary into RAM
	if err := reader.loadSummary(); err != nil {
		return nil, err
	}
	// load filter into RAM
	if err := reader.loadFilter(); err != nil {
		reader.filterSegment = nil
	}

	// load merkle tree into RAM
	if err := reader.loadMerkleTree(); err != nil {
		return nil, err
	}
	return reader, nil
}

// loadSummary reads and decodes summary segment, which maps key ranges into index blocks
func (r *SSTableReader) loadSummary() error {
	data, err := r.storage.ReadSegment(config.SegmentSummary, r.footer.SummaryHandler.Offset, r.footer.SummaryHandler.Size)
	if err != nil {
		return err
	}
	summary, err := DecodeSummarySegment(data)
	if err != nil {
		return err
	}
	r.summarySegment = summary
	return nil
}

// loadFilter reads and decodes the filter segment -> filter allows quickly checking whether a key exists or nott
func (r *SSTableReader) loadFilter() error {
	if r.footer.FilterHandler.Size == 0 {
		return errors.New("no filter segment")
	}
	data, err := r.storage.ReadSegment(config.SegmentFilter, r.footer.FilterHandler.Offset, r.footer.FilterHandler.Size)
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
	data, err := r.storage.ReadSegment(config.SegmentMetadata, r.footer.MetaDataHandler.Offset, r.footer.MetaDataHandler.Size)
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

	indexBlockSize := r.config.SSTable.IndexSegment.IndexBlockSize

	offset := r.footer.IndexHandler.Offset +
		uint64(blockNumber)*uint64(indexBlockSize)
	data, err := r.storage.ReadSegment(
		config.SegmentIndex,
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

	// step 2
	indexBlockNum := r.summarySegment.FindIndexBlockNumber(key)
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
	if _, err := os.Stat(r.filePath + ".data"); err == nil {
		dataFilePath = r.filePath + ".data"
	}
	blockKey := block.BlockKey{
		FilePath: dataFilePath,
		Offset:   dataBlockIdx,
	}
	blockData, err := r.blockManager.Read(blockKey)
	if err != nil {
		return nil, err
	}

	// step 5
	iterator, err := NewDataBlockIteratorRaw(blockData, int(r.footer.RestartInterval), r.footer.EncodingType)
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
		Timestamp: rec.Timestamp,
		Tombstone: rec.Tombstone,
		Key:       rec.Key,
		Value:     rec.Value,
	}, nil
}
