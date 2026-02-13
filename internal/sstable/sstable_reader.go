package sstable

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
)

type SSTableReader struct {
	storage        SegmentStorage
	blockManager   *block.BlockManager
	filePath       string
	footer         *Footer
	summarySegment *SummarySegment
	filterSegment  *FilterSegment
	merkleTree     *MerkleTree
	config         *config.Config
}

func NewSSTableReader(filePath string, blockManager *block.BlockManager, cfg *config.Config) (*SSTableReader, error) {
	storage, err := OpenStorage(filePath, cfg)
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
	footerData, err := storage.ReadSegment(config.SegmentFooter, offset, FooterSize)
	if err != nil {
		return nil, err
	}
	footer := &Footer{}
	footer.Decode(footerData)
	if err := footer.Validate(); err != nil {
		return nil, err
	}
	reader := &SSTableReader{
		storage:      storage,
		blockManager: blockManager,
		filePath:     filePath,
		footer:       footer,
		config:       cfg,
	}
	if err := reader.loadSummary(); err != nil {
		return nil, err
	}
	if err := reader.loadFilter(); err != nil {
		reader.filterSegment = nil
	}
	if err := reader.loadMerkleTree(); err != nil {
		return nil, err
	}
	return reader, nil
}

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

func (r *SSTableReader) loadIndexBlock(blockNumber int) (*IndexBlock, error) {
	if r.footer == nil {
		return nil, errors.New("no footer")
	}
	indexBlockSize := r.config.SSTable.IndexSegment.IndexBlockSize
	offset := blockNumber
	var file *os.File
	switch s := r.storage.(type) {
	case *SingleFileStorage:
		file = s.File()
	case *MultiFileStorage:
		var err error
		file, err = s.getOrOpenFile(config.SegmentIndex)
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("unsupported storage type")
	}
	data := make([]byte, indexBlockSize)
	if _, err := file.ReadAt(data, int64(offset)); err != nil {
		return nil, err
	}
	fmt.Println(data)
	block, err := DecodeIndexBlock(data)
	if err != nil {
		return nil, err
	}
	return block, nil
}

func (r *SSTableReader) Get(key []byte) (*Record, error) {
	if r.filterSegment != nil && r.filterSegment.Filter() != nil {
		if !r.filterSegment.Filter().MightContain(key) {
			return nil, nil
		}
	}
	indexBlockNum := r.summarySegment.FindIndexBlockNumber(key)
	fmt.Println(indexBlockNum)
	if indexBlockNum < 0 {
		return nil, nil
	}
	indexBlock, err := r.loadIndexBlock(int(r.footer.IndexHandler.Offset) + indexBlockNum*r.config.SSTable.IndexSegment.IndexBlockSize - indexBlockNum)
	if err != nil {
		return nil, err
	}
	fmt.Println("Trazim kljuc doso sam ovd")
	fmt.Println(indexBlock)
	entryIdx := indexBlock.FindBlock(key)
	fmt.Println("Entry Index", entryIdx)
	if entryIdx < 0 {
		return nil, nil
	}
	dataBlockIdx := indexBlock.Entries[entryIdx].BlockIndex
	fmt.Println("Data Block Index", dataBlockIdx)
	blockKey := block.BlockKey{
		FilePath: r.filePath,
		Offset:   dataBlockIdx,
	}
	blockData, err := r.blockManager.Read(blockKey)
	if err != nil {
		return nil, err
	}
	fmt.Println("BLOCK", blockData)
	iterator, err := NewDataBlockIterator(blockData)
	if err != nil {
		return nil, err
	}
	iterator.Rewind()
	fmt.Println("Iterator", iterator.Current().Key)
	defer iterator.Close()
	fmt.Println("Prosao sam close")
	if err := iterator.Seek(key); err != nil {
		fmt.Println("Seek", err)
		return nil, err
	}
	if bytes.Equal(iterator.Current().Key, key) {
		return &Record{
			Timestamp: iterator.Timestamp(),
			Tombstone: iterator.Tombstone(),
			Key:       iterator.Key(),
			Value:     iterator.Value(),
		}, nil
	}
	return nil, nil
}
