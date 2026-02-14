package sstable

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"os"
)

type IndexEntry struct {
	Key        []byte
	BlockIndex uint32
}

func (entry *IndexEntry) EncodedSize() int {
	keyLen := uint64(len(entry.Key))
	buf := make([]byte, binary.MaxVarintLen64)
	varintLen := binary.PutUvarint(buf, keyLen)
	return varintLen + len(entry.Key) + 4
}

func (entry *IndexEntry) EncodeTo(buf []byte) int {
	keyLen := uint64(len(entry.Key))
	pos := 0
	n := binary.PutUvarint(buf[pos:], keyLen)
	pos += n
	copy(buf[pos:], entry.Key)
	pos += len(entry.Key)
	binary.LittleEndian.PutUint32(buf[pos:], entry.BlockIndex)
	pos += 4
	return pos
}

func (entry *IndexEntry) EncodeIndexEntry() []byte {
	size := entry.EncodedSize()
	buf := make([]byte, size)
	entry.EncodeTo(buf)
	return buf
}

func DecodeIndexEntry(buf []byte) (*IndexEntry, int, error) {
	if len(buf) < 5 {
		return nil, 0, errors.New("Buffer too small")
	}
	pos := 0
	keyLength, n := binary.Uvarint(buf[pos:])
	if n <= 0 {
		return nil, 0, errors.New("Invalid index key length")
	}
	pos += n
	if pos+int(keyLength)+4 > len(buf) {
		return nil, 0, errors.New("Buffer too small")
	}
	key := make([]byte, keyLength)
	copy(key, buf[pos:pos+int(keyLength)])
	pos += int(keyLength)
	blockIndex := binary.LittleEndian.Uint32(buf[pos:])
	pos += 4
	return &IndexEntry{
		Key:        key,
		BlockIndex: blockIndex,
	}, pos, nil
}

type IndexBlock struct {
	Entries []IndexEntry
}

func NewIndexBlock() *IndexBlock {
	return &IndexBlock{
		Entries: make([]IndexEntry, 0, 256),
	}
}

func (block *IndexBlock) AddEntry(entry IndexEntry) {
	block.Entries = append(block.Entries, entry)
}

func (block *IndexBlock) EncodeIndexBlock() []byte {
	totalSize := block.Size()
	buf := make([]byte, totalSize)
	pos := 0
	binary.LittleEndian.PutUint32(buf[pos:], uint32(len(block.Entries)))
	pos += 4
	for i := range block.Entries {
		n := block.Entries[i].EncodeTo(buf[pos:])
		pos += n
	}
	crc := crc32.ChecksumIEEE(buf[:pos])
	binary.LittleEndian.PutUint32(buf[pos:], crc)
	pos += 4
	return buf
}

func DecodeIndexBlock(buf []byte) (*IndexBlock, error) {
	numEntries := binary.LittleEndian.Uint32(buf[0:4])
	if numEntries == 0 {
		return &IndexBlock{Entries: []IndexEntry{}}, nil
	}
	pos := 4
	entries := make([]IndexEntry, 0, numEntries)
	for i := 0; i < int(numEntries); i++ {
		entry, n, err := DecodeIndexEntry(buf[pos:])
		if err != nil {
			return nil, err
		}
		entries = append(entries, *entry)
		pos += n
	}
	return &IndexBlock{
		Entries: entries,
	}, nil
}

func (block *IndexBlock) FindBlock(key []byte) int {
	if len(block.Entries) == 0 {
		return -1
	}
	if bytes.Compare(key, block.Entries[0].Key) < 0 {
		return -1
	}
	lastIdx := len(block.Entries) - 1
	if bytes.Compare(key, block.Entries[lastIdx].Key) >= 0 {
		return lastIdx
	}
	left := 0
	right := lastIdx
	result := -1
	for left <= right {
		mid := left + (right-left)/2
		cmp := bytes.Compare(key, block.Entries[mid].Key)
		if cmp < 0 {
			right = mid - 1
		} else if cmp == 0 {
			return mid
		} else {
			result = mid
			left = mid + 1
		}
	}
	return result
}

func (block *IndexBlock) Size() int {
	size := 4
	for i := range block.Entries {
		size += block.Entries[i].EncodedSize()
	}
	size += 4
	return size
}

func (block *IndexBlock) WriteToFile(file *os.File) (int, error) {
	data := block.EncodeIndexBlock()
	n, err := file.Write(data)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func ReadFromFile(file *os.File, offset uint64, size int) (*IndexBlock, error) {
	if _, err := file.Seek(int64(offset), 0); err != nil {
		return nil, err
	}
	data := make([]byte, size)
	if _, err := file.Read(data); err != nil {
		return nil, err
	}
	return DecodeIndexBlock(data)
}

func (block *IndexBlock) AddFromDataBlock(dataBlock *DataBlockBuilder, blockIdx uint32) {
	firstKey := dataBlock.firstKey
	if firstKey == nil || len(firstKey) == 0 {
		return
	}
	keyCopy := make([]byte, len(firstKey))
	copy(keyCopy, firstKey)
	block.AddEntry(IndexEntry{
		Key:        keyCopy,
		BlockIndex: blockIdx,
	})
}

type IndexSegment struct {
	Blocks       []*IndexBlock
	BlockOffsets []uint32
}

func NewIndexSegment() *IndexSegment {
	return &IndexSegment{
		Blocks:       make([]*IndexBlock, 0),
		BlockOffsets: make([]uint32, 0),
	}
}

func (seg *IndexSegment) AddBlock(block *IndexBlock) {
	seg.Blocks = append(seg.Blocks, block)
}

func (seg *IndexSegment) WriteToFile(file *os.File) ([]uint32, error) {
	offsets := make([]uint32, len(seg.Blocks))
	var currentOffset int32 = 0
	for i, block := range seg.Blocks {
		offsets[i] = uint32(currentOffset)
		data := block.EncodeIndexBlock()
		n, err := file.Write(data)
		if err != nil {
			return nil, err
		}
		currentOffset += int32(n)
		for j := range block.Entries {
			block.Entries[j].BlockIndex = offsets[i]
		}
	}
	seg.BlockOffsets = offsets
	return offsets, nil
}

func (seg *IndexSegment) ReadBlockFromFile(file *os.File, offset uint64, size int) (*IndexBlock, error) {
	if _, err := file.Seek(int64(offset), 0); err != nil {
		return nil, err
	}
	data := make([]byte, size)
	if _, err := file.Read(data); err != nil {
		return nil, err
	}
	return DecodeIndexBlock(data)
}

func (seg *IndexSegment) AddEntryToBlock(entry IndexEntry, blockSize int) {
	if len(seg.Blocks) == 0 || len(seg.Blocks[len(seg.Blocks)-1].Entries) >= blockSize {
		seg.Blocks = append(seg.Blocks, NewIndexBlock())
	}
	seg.Blocks[len(seg.Blocks)-1].AddEntry(entry)
}

func (seg *IndexSegment) GetBlockSizes() []int {
	sizes := make([]int, len(seg.Blocks))
	for i, block := range seg.Blocks {
		sizes[i] = block.Size()
	}
	return sizes
}
