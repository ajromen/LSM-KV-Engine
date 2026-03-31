package sstable

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
)

type TTLEntry struct {
	Key       []byte
	ExpiresAt int64
}

func (entry *TTLEntry) EncodedSize() int {
	keyLen := uint64(len(entry.Key))
	buf := make([]byte, binary.MaxVarintLen64)
	variantLen := binary.PutUvarint(buf, keyLen)
	return variantLen + len(entry.Key) + 8
}

// EncodeTo SERIALIZES THE TTL INDEX ENTRY INTO THE PROVIDED BUFFER, RETURNS THE NUMBER OF BYTES WRITTEN
func (entry *TTLEntry) EncodeTo(buf []byte) int {
	keyLen := uint64(len(entry.Key))
	pos := 0

	// encode key-length as varint
	n := binary.PutUvarint(buf[pos:], keyLen)
	pos += n

	// encode key bytes
	copy(buf[pos:], entry.Key)
	pos += len(entry.Key)

	// encode ttl
	binary.LittleEndian.PutUint64(buf[pos:], uint64(entry.ExpiresAt))
	pos += 8

	return pos
}

// EncodeTTLIndexEntry ALLOCATES A NEW BUFFER AND ENCODES THE ENTRY INTO IT
func (entry *TTLEntry) EncodeTTLIndexEntry() []byte {
	size := entry.EncodedSize()
	buf := make([]byte, size)
	entry.EncodeTo(buf)
	return buf
}

// DecodeTTLIndexEntry DESERIALIZES TTL INDEX ENTRY FROM GIVEN BUFFER -> RETURNS DECODED ENTRY, BYTES CONSUMED AND ERROR IF ANY
func DecodeTTLIndexEntry(buf []byte) (*TTLEntry, int, error) {
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
	if pos+int(keyLength)+8 > len(buf) {
		return nil, 0, errors.New("buffer too small")
	}

	// decode key
	key := make([]byte, keyLength)
	copy(key, buf[pos:pos+int(keyLength)])
	pos += int(keyLength)

	// decode index of block in data segment
	expiresAt := binary.LittleEndian.Uint64(buf[pos:])
	pos += 8
	return &TTLEntry{
		Key:       key,
		ExpiresAt: int64(expiresAt),
	}, pos, nil
}

type TTLIndexBlock struct {
	Entries  []TTLEntry
	RealSize uint32
}

// NewTTLIndexBlock creates an empty index block with prealocated capacity -> 256!= ttl-index-block-size
func NewTTLIndexBlock() *TTLIndexBlock {
	return &TTLIndexBlock{
		Entries:  make([]TTLEntry, 0, 256),
		RealSize: 4,
	}
}

// Add appends a ttl index entry to block
func (block *TTLIndexBlock) Add(entry TTLEntry) {
	block.Entries = append(block.Entries, entry)
	block.RealSize += uint32(entry.EncodedSize())
}

func (block *TTLIndexBlock) Encode(maxBlockSize uint64) []byte {
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

	return buf
}

func DecodeTTLIndexBlock(buf []byte) (*TTLIndexBlock, error) {
	// read number of entries
	numEntries := binary.LittleEndian.Uint32(buf[0:4])
	if numEntries == 0 {
		return &TTLIndexBlock{Entries: []TTLEntry{}}, nil
	}
	pos := 4

	// read entries
	entries := make([]TTLEntry, 0, numEntries)
	for i := 0; i < int(numEntries); i++ {
		entry, n, err := DecodeTTLIndexEntry(buf[pos:])
		if err != nil {
			return nil, err
		}
		entries = append(entries, *entry)
		pos += n
	}

	return &TTLIndexBlock{
		Entries: entries,
	}, nil
}

type TTLIndexSegment struct {
	Blocks            []*TTLIndexBlock
	TTLIndexBlockSize uint64
}

func NewTTLIndexSegment(indexBlockSize uint64) *TTLIndexSegment {
	return &TTLIndexSegment{
		Blocks:            make([]*TTLIndexBlock, 0),
		TTLIndexBlockSize: indexBlockSize,
	}
}

func (seg *TTLIndexSegment) Add(block *TTLIndexBlock) {
	seg.Blocks = append(seg.Blocks, block)

}

func (seg *TTLIndexSegment) AddEntryToBlock(entry TTLEntry, blockSize int) {
	if len(seg.Blocks) == 0 || len(seg.Blocks[len(seg.Blocks)-1].Entries) >= blockSize {
		seg.Blocks = append(seg.Blocks, NewTTLIndexBlock())
	}
	seg.Blocks[len(seg.Blocks)-1].Add(entry)
}

// GetBlockSizes RETURNS ENCODED SIZE OF EACH TTL INDEX BLOCK IN INDEX SEGMENT
func (seg *TTLIndexSegment) GetBlockSizes() []uint32 {
	sizes := make([]uint32, len(seg.Blocks))
	for i, block := range seg.Blocks {
		sizes[i] = block.RealSize
	}
	return sizes
}
