package sstable

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"

	"github.com/ajromen/LSM-KV-Engine/internal/shared"
)

/*
┌──────────────────────────────────────────────────────────────────────────────┐
│                               INDEX BLOCK                                    │
│																			   │
│	Number of entries (uint32, little-endian)								   │
│																			   │
│  ┌────────────────────────────────────────────────────────────────────────┐  │
│  │  Entry 1                                                               │  │
│  │    - key_len (uvarint)                                                 │  │
│  │    - key bytes                                                         │  │
│  │    - block_offset (uint32, little-endian)                              │  │
│  │                                                                        │  │
│  │  Entry 2                                                               │  │
│  │    - key_len (uvarint)                                                 │  │
│  │    - key bytes                                                         │  │
│  │    - block_offset (uint32, little-endian)                              │  │
│  │                                                                        │  │
│  │  ...                                                                   │  │
│  └────────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
│                                                                              │
│  CRC32 Checksum (uint32, little-endian)                                      │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘

NOTES:
Entries are sorted by key in ascending order.
Each entry maps the FIRST key of a Data Block to the block's file offset.
Binary search is performed over the entries to locate the correct data block.
CRC32 is calculated over the entire block except the last 4 bytes (CRC itself).
block_offset points to the beginning of the corresponding Data Block on disk.
*/

// IndexBlock IS A GROP OF INDEX ENTRY RECORDS
type IndexBlock struct {
	Entries  []shared.IndexEntry
	RealSize uint32 // where numberOfEntries + index entries end
}

// NewIndexBlock CREATES AN EMPTY INDEX BLOCK WITH PREALLOCATED CAPACITY -> IMPORTANT: 256 != INDEX-BLOCK-SIZE
func NewIndexBlock() *IndexBlock {
	return &IndexBlock{
		Entries:  make([]shared.IndexEntry, 0, 256),
		RealSize: 4,
	}
}

// AddEntry APPENDS AN INDEX ENTRY TO THE BLOCK
func (block *IndexBlock) AddEntry(entry shared.IndexEntry) {
	block.Entries = append(block.Entries, entry)
	block.RealSize += uint32(entry.EncodedSize())
}

// EncodeIndexBlock SERIALIZES THE INDEX BLOCK
func (block *IndexBlock) EncodeIndexBlock(maxBlockSize uint64) []byte {
	buf := make([]byte, maxBlockSize)
	pos := 0

	// write number of entries
	binary.LittleEndian.PutUint32(buf[pos:], uint32(len(block.Entries)))
	pos += 4

	// write entries
	for i := range block.Entries {
		n := block.Entries[i].EncodeTo(buf[pos:])
		pos += n
	}

	// write crc
	crcPos := maxBlockSize - 4
	crc := crc32.ChecksumIEEE(buf[:crcPos])
	binary.LittleEndian.PutUint32(buf[crcPos:], crc)
	pos += 4
	return buf
}

// DecodeIndexBlock DESERIALIZES THE INDEX BLOCK
func DecodeIndexBlock(buf []byte) (*IndexBlock, error) {
	// read number of entries
	numEntries := binary.LittleEndian.Uint32(buf[0:4])
	if numEntries == 0 {
		return &IndexBlock{Entries: []shared.IndexEntry{}}, nil
	}
	pos := 4

	// read entries
	entries := make([]shared.IndexEntry, 0, numEntries)
	for i := 0; i < int(numEntries); i++ {
		entry, n, err := shared.DecodeIndexEntry(buf[pos:])
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

// FindBlock PERFORMS BINARY SEARCH IN INDEX BLOCK AND RETURNS THE INDEX OF THE BLOCK IN DATA THAT MAY CONTAIN THE GIVEN KEY
func (block *IndexBlock) FindBlock(key []byte) int {
	if len(block.Entries) == 0 {
		return -1
	}
	if bytes.Compare(key, block.Entries[0].Key) < 0 {
		return 0
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

// AddFromDataBlock CREATES AN INDEX ENTRY AND ADDS IT TO BLOCK FROM DATA BLOCK BUILDER AND INDEX OF GIVEN BLOCK
func (block *IndexBlock) AddFromDataBlock(dataBlock *DataBlockBuilder, blockIdx uint32) {
	firstKey := dataBlock.firstKey
	if firstKey == nil || len(firstKey) == 0 {
		return
	}
	keyCopy := make([]byte, len(firstKey))
	copy(keyCopy, firstKey)
	block.AddEntry(shared.IndexEntry{
		Key:        keyCopy,
		BlockIndex: blockIdx,
	})
}

// IndexSegment REPRESENTS MULTIPLE INDEX BLOCK STORED SEQUENTIALLY
type IndexSegment struct {
	Blocks         []*IndexBlock
	BlockOffsets   []uint32
	IndexBlockSize uint64
}

func NewIndexSegment(indexBlockSize uint64) *IndexSegment {
	return &IndexSegment{
		Blocks:         make([]*IndexBlock, 0),
		BlockOffsets:   make([]uint32, 0),
		IndexBlockSize: indexBlockSize,
	}
}

// AddBlock APPENDS NEW INDEX BLOCK TO INDEX SEGMENT
func (seg *IndexSegment) AddBlock(block *IndexBlock) {
	seg.Blocks = append(seg.Blocks, block)
}

// AddEntryToBlock ADDS ENTRY AND CREATES A NEW BLOCK IF THE CURRENT ONE IS FULL
func (seg *IndexSegment) AddEntryToBlock(entry shared.IndexEntry, blockSize int) {
	if len(seg.Blocks) == 0 || len(seg.Blocks[len(seg.Blocks)-1].Entries) >= blockSize {
		seg.Blocks = append(seg.Blocks, NewIndexBlock())
	}
	seg.Blocks[len(seg.Blocks)-1].AddEntry(entry)
}

// GetBlockSizes RETURNS ENCODED SIZE OF EACH INDEX BLOCK IN INDEX SEGMENT
func (seg *IndexSegment) GetBlockSizes() []uint32 {
	sizes := make([]uint32, len(seg.Blocks))
	for i, block := range seg.Blocks {
		sizes[i] = block.RealSize
	}
	return sizes
}
