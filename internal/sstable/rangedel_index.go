package sstable

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"

	"github.com/ajromen/LSM-KV-Engine/internal/shared"
)

type RangeDelIndexBlock struct {
	Entries  []shared.RangeDelEntry
	RealSize uint32
}

func NewRangeDelIndexBlock() *RangeDelIndexBlock {
	return &RangeDelIndexBlock{
		Entries:  make([]shared.RangeDelEntry, 0, 256),
		RealSize: 4,
	}
}

func (block *RangeDelIndexBlock) Add(entry shared.RangeDelEntry) {
	block.Entries = append(block.Entries, entry)
	block.RealSize += uint32(entry.EncodedSize())
}

func (block *RangeDelIndexBlock) Encode(maxBlockSize uint64) []byte {
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

func DecodeRangeDelIndexBlock(buf []byte) (*RangeDelIndexBlock, error) {
	// read number of entries
	numEntries := binary.LittleEndian.Uint32(buf[0:4])
	if numEntries == 0 {
		return &RangeDelIndexBlock{Entries: []shared.RangeDelEntry{}}, nil
	}
	pos := 4

	// read entries
	entries := make([]shared.RangeDelEntry, 0, numEntries)
	for i := 0; i < int(numEntries); i++ {
		entry, n, err := shared.DecodeRangeDelEntry(buf[pos:])
		if err != nil {
			return nil, err
		}
		entries = append(entries, *entry)
		pos += n
	}

	return &RangeDelIndexBlock{
		Entries: entries,
	}, nil
}

func (block *RangeDelIndexBlock) MaxCoveringRange(key []byte, seqId uint64) uint64 {
	n := len(block.Entries)
	if n == 0 {
		return 0
	}
	left := 0
	right := n - 1
	pos := n
	for left <= right {
		mid := left + (right-left)/2

		if bytes.Compare(block.Entries[mid].StartKey, key) > 0 {
			pos = mid
			right = mid - 1
		} else {
			left = mid + 1
		}
	}
	maxSeq := uint64(0)
	for i := pos - 1; i >= 0; i-- {
		e := block.Entries[i]
		if bytes.Compare(e.EndKey, key) <= 0 {
			break
		}
		if e.SeqId > seqId && e.SeqId > maxSeq {
			maxSeq = e.SeqId
		}
	}
	return maxSeq
}

type RangeDelIndexSegment struct {
	Blocks                 []*RangeDelIndexBlock
	FirstKeys              [][]byte
	RangeDelIndexBlockSize uint64
}

func NewRangeDelIndexSegment(indexBlockSize uint64) *RangeDelIndexSegment {
	return &RangeDelIndexSegment{
		Blocks:                 make([]*RangeDelIndexBlock, 0),
		FirstKeys:              make([][]byte, 0),
		RangeDelIndexBlockSize: indexBlockSize,
	}
}

func (seg *RangeDelIndexSegment) Add(block *RangeDelIndexBlock) {
	seg.Blocks = append(seg.Blocks, block)
}

func (seg *RangeDelIndexSegment) AddEntryToBlock(entry shared.RangeDelEntry, blockSize int) {
	if len(seg.Blocks) == 0 || len(seg.Blocks[len(seg.Blocks)-1].Entries) >= blockSize {
		seg.Blocks = append(seg.Blocks, NewRangeDelIndexBlock())
	}
	seg.Blocks[len(seg.Blocks)-1].Add(entry)
	if len(seg.Blocks[len(seg.Blocks)-1].Entries) == 1 {
		seg.FirstKeys = append(seg.FirstKeys, entry.StartKey)
	}
}

func (seg *RangeDelIndexSegment) GetBlockSizes() []uint32 {
	sizes := make([]uint32, len(seg.Blocks))
	for i, block := range seg.Blocks {
		sizes[i] = block.RealSize
	}
	return sizes
}

func (seg *RangeDelIndexSegment) FindBlockIndex(key []byte) int {
	n := len(seg.FirstKeys)
	if n == 0 {
		return -1
	}
	left := 0
	right := n - 1
	for left <= right {
		mid := left + (right-left)/2
		cmp := bytes.Compare(seg.FirstKeys[mid], key)
		if cmp <= 0 {
			left = mid + 1
		} else {
			right = mid - 1
		}
	}
	if right < 0 {
		return -1
	}
	return right
}

func (seg *RangeDelIndexSegment) MaxCoveringSeqId(key []byte, seqId uint64) uint64 {
	idx := seg.FindBlockIndex(key)
	if idx < 0 || idx >= len(seg.Blocks) {
		return 0
	}
	maxSeq := seg.Blocks[idx].MaxCoveringRange(key, seqId)
	if idx > 0 {
		seq := seg.Blocks[idx-1].MaxCoveringRange(key, seqId)
		if seq > maxSeq {
			maxSeq = seq
		}
	}
	return maxSeq
}
