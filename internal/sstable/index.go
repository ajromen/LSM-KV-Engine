package sstable

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"os"
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

// IndexEntry IS A SINGLE RECORD WRITTEN INTO INDEX BLOCK -> IT MAPS A KEY TO THE OFFSET OF A DATA BLOCK IN THE SSTABLE FILE
type IndexEntry struct {
	Key        []byte // first key of data block
	BlockIndex uint32 // file offset of data block
}

// EncodedSize RETURNS THE NUMBER OF BYTES REQUIRED TO ENCODE THIS ENTRY
func (entry *IndexEntry) EncodedSize() int {
	keyLen := uint64(len(entry.Key))
	buf := make([]byte, binary.MaxVarintLen64)
	varintLen := binary.PutUvarint(buf, keyLen)
	return varintLen + len(entry.Key) + 4
}

// EncodeTo SERIALIZES THE INDEX ENTRY INTO THE PROVIDED BUFFER, RETURNS THE NUMBER OF BYTES WRITTEN
func (entry *IndexEntry) EncodeTo(buf []byte) int {
	keyLen := uint64(len(entry.Key))
	pos := 0

	// encode key-length as varint
	n := binary.PutUvarint(buf[pos:], keyLen)
	pos += n

	// encode key bytes
	copy(buf[pos:], entry.Key)
	pos += len(entry.Key)

	// encode index of block in data segment
	binary.LittleEndian.PutUint32(buf[pos:], entry.BlockIndex)
	pos += 4
	return pos
}

// EncodeIndexEntry ALLOCATES A NEW BUFFER AND ENCODES THE ENTRY INTO IT
func (entry *IndexEntry) EncodeIndexEntry() []byte {
	size := entry.EncodedSize()
	buf := make([]byte, size)
	entry.EncodeTo(buf)
	return buf
}

// DecodeIndexEntry DESERIALIZES INDEX ENTRY FROM GIVEN BUFFER -> RETURNS DECODED ENTRY, BYTES CONSUMED AND ERROR IF ANY
func DecodeIndexEntry(buf []byte) (*IndexEntry, int, error) {
	if len(buf) < 5 {
		return nil, 0, errors.New("buffer too small")
	}
	pos := 0

	// decode key length
	keyLength, n := binary.Uvarint(buf[pos:])
	if n <= 0 {
		return nil, 0, errors.New("invalid index key length")
	}
	pos += n
	if pos+int(keyLength)+4 > len(buf) {
		return nil, 0, errors.New("buffer too small")
	}

	// decode key
	key := make([]byte, keyLength)
	copy(key, buf[pos:pos+int(keyLength)])
	pos += int(keyLength)

	// decode index of block in data segment
	blockIndex := binary.LittleEndian.Uint32(buf[pos:])
	pos += 4
	return &IndexEntry{
		Key:        key,
		BlockIndex: blockIndex,
	}, pos, nil
}

// IndexBlock IS A GROP OF INDEX ENTRY RECORDS
type IndexBlock struct {
	Entries  []IndexEntry
	RealSize uint32 // where numberOfEntries + index entries end
}

// NewIndexBlock CREATES AN EMPTY INDEX BLOCK WITH PREALLOCATED CAPACITY -> IMPORTANT: 256 != INDEX-BLOCK-SIZE
func NewIndexBlock() *IndexBlock {
	return &IndexBlock{
		Entries:  make([]IndexEntry, 0, 256),
		RealSize: 4,
	}
}

// AddEntry APPENDS AN INDEX ENTRY TO THE BLOCK
func (block *IndexBlock) AddEntry(entry IndexEntry) {
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
		return &IndexBlock{Entries: []IndexEntry{}}, nil
	}
	pos := 4

	// read entries
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

// FindBlock PERFORMS BINARY SEARCH IN INDEX BLOCK AND RETURNS THE INDEX OF THE BLOCK IN DATA THAT MAY CONTAIN THE GIVEN KEY
//
//goland:noinspection GoRedundantElseInIf
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

// ReadFromFile READS INDEX BLOCK FROM FILE -> not used
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

// AddFromDataBlock CREATES AN INDEX ENTRY AND ADDS IT TO BLOCK FROM DATA BLOCK BUILDER AND INDEX OF GIVEN BLOCK
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
func (seg *IndexSegment) AddEntryToBlock(entry IndexEntry, blockSize int) {
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
