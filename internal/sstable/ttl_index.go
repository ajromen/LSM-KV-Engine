package sstable

import (
	"encoding/binary"
	"hash/crc32"

	"github.com/ajromen/LSM-KV-Engine/internal/shared"
)

type TTLIndexBlock struct {
	Entries  []shared.TTLEntry
	RealSize uint32
}

// NewTTLIndexBlock creates an empty index block with prealocated capacity -> 256!= ttl-index-block-size
func NewTTLIndexBlock() *TTLIndexBlock {
	return &TTLIndexBlock{
		Entries:  make([]shared.TTLEntry, 0, 256),
		RealSize: 4,
	}
}

// Add appends a ttl index entry to block
func (block *TTLIndexBlock) Add(entry shared.TTLEntry) {
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
		return &TTLIndexBlock{Entries: []shared.TTLEntry{}}, nil
	}
	pos := 4

	// read entries
	entries := make([]shared.TTLEntry, 0, numEntries)
	for i := 0; i < int(numEntries); i++ {
		entry, n, err := shared.DecodeTTLIndexEntry(buf[pos:])
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

func (seg *TTLIndexSegment) AddEntryToBlock(entry shared.TTLEntry, blockSize int) {
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
