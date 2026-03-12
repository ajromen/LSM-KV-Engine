//	type SummaryEntry struct {
//		Key           []byte //first key of index block
//		Offset        uint64 //offset of the index block
//		NumEntries    uint32 //number of entries in the index block
//		MinDataOffset uint64 //minimum data block offset in the index block
//		MaxDataOffset uint64 //maximum data block offset in the index block
//	}
//
//	type SummarySegment struct {
//		MinKey           []byte         //min key (border of index file)
//		Entries          []SummaryEntry //entries (first record of every samplingdegree-nth block
//		MaxKey           []byte         //max key (rigiht border of index file)
//		SamplingDegree   uint32         //for entries pise
//		TotalIndexBlocks uint32         //number of index blocks -> maybe need for footer
//	}
package sstable

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"os"
)

type SummaryEntry struct {
	Key              []byte
	IndexBlockOffset uint64
}

type SummarySegment struct {
	MinKey           []byte
	Entries          []SummaryEntry
	MaxKey           []byte
	SamplingDegree   uint32
	TotalIndexBlocks uint32
}

func NewSummarySegment(samplingDegree uint32) *SummarySegment {
	return &SummarySegment{
		Entries:        make([]SummaryEntry, 0),
		SamplingDegree: samplingDegree,
	}
}

////////////////////////////////////////////////////////////
// ENCODING / DECODING (same pattern as filter/index)
////////////////////////////////////////////////////////////

func (s *SummarySegment) Encode() []byte {
	size := s.EstimatedSize()
	buf := make([]byte, size)
	pos := 0

	// MinKey
	n := binary.PutUvarint(buf[pos:], uint64(len(s.MinKey)))
	pos += n
	copy(buf[pos:], s.MinKey)
	pos += len(s.MinKey)

	// MaxKey
	n = binary.PutUvarint(buf[pos:], uint64(len(s.MaxKey)))
	pos += n
	copy(buf[pos:], s.MaxKey)
	pos += len(s.MaxKey)

	// SamplingDegree
	binary.LittleEndian.PutUint32(buf[pos:], s.SamplingDegree)
	pos += 4

	// TotalIndexBlocks
	binary.LittleEndian.PutUint32(buf[pos:], s.TotalIndexBlocks)
	pos += 4

	// Number of entries
	binary.LittleEndian.PutUint32(buf[pos:], uint32(len(s.Entries)))
	pos += 4

	// Entries
	for _, e := range s.Entries {
		n := binary.PutUvarint(buf[pos:], uint64(len(e.Key)))
		pos += n
		copy(buf[pos:], e.Key)
		pos += len(e.Key)

		binary.LittleEndian.PutUint64(buf[pos:], e.IndexBlockOffset)
		pos += 8
	}

	crc := crc32.ChecksumIEEE(buf[:pos])
	binary.LittleEndian.PutUint32(buf[pos:], crc)

	return buf[:pos+4]
}

func DecodeSummarySegment(buf []byte) (*SummarySegment, error) {
	if len(buf) < 4 {
		return nil, errors.New("buffer too small")
	}

	crcPos := len(buf) - 4
	expected := binary.LittleEndian.Uint32(buf[crcPos:])
	actual := crc32.ChecksumIEEE(buf[:crcPos])
	if expected != actual {
		return nil, errors.New("summary CRC mismatch")
	}

	pos := 0

	// MinKey
	minLen, n := binary.Uvarint(buf[pos:])
	if n <= 0 {
		return nil, errors.New("invalid minKey length")
	}
	pos += n
	minKey := make([]byte, minLen)
	copy(minKey, buf[pos:pos+int(minLen)])
	pos += int(minLen)

	// MaxKey
	maxLen, n := binary.Uvarint(buf[pos:])
	if n <= 0 {
		return nil, errors.New("invalid maxKey length")
	}
	pos += n
	maxKey := make([]byte, maxLen)
	copy(maxKey, buf[pos:pos+int(maxLen)])
	pos += int(maxLen)

	sampling := binary.LittleEndian.Uint32(buf[pos:])
	pos += 4

	totalBlocks := binary.LittleEndian.Uint32(buf[pos:])
	pos += 4

	numEntries := binary.LittleEndian.Uint32(buf[pos:])
	pos += 4

	entries := make([]SummaryEntry, 0, numEntries)

	for i := 0; i < int(numEntries); i++ {
		keyLen, n := binary.Uvarint(buf[pos:])
		if n <= 0 {
			return nil, errors.New("invalid entry key length")
		}
		pos += n

		key := make([]byte, keyLen)
		copy(key, buf[pos:pos+int(keyLen)])
		pos += int(keyLen)

		offset := binary.LittleEndian.Uint64(buf[pos:])
		pos += 8

		entries = append(entries, SummaryEntry{
			Key:              key,
			IndexBlockOffset: offset,
		})
	}

	return &SummarySegment{
		MinKey:           minKey,
		MaxKey:           maxKey,
		SamplingDegree:   sampling,
		TotalIndexBlocks: totalBlocks,
		Entries:          entries,
	}, nil
}

////////////////////////////////////////////////////////////
// SEARCH FUNCTIONS
////////////////////////////////////////////////////////////

// Returns index block number where key should be
func (s *SummarySegment) FindIndexBlockNumber(key []byte) int {
	left := 0
	right := len(s.Entries) - 1
	result := -1

	for left <= right {
		mid := left + (right-left)/2
		cmp := bytes.Compare(key, s.Entries[mid].Key)

		if cmp < 0 {
			right = mid - 1
		} else if cmp == 0 {
			return mid * int(s.SamplingDegree)
		} else {
			result = mid
			left = mid + 1
		}
	}

	if result == -1 {
		return 0
	}

	return result * int(s.SamplingDegree)
}

func (s *SummarySegment) FindBlockRange(startKey, endKey []byte) (int, int) {
	startBlock := s.FindIndexBlockNumber(startKey)
	endBlock := s.FindIndexBlockNumber(endKey)

	if startBlock == -1 || endBlock == -1 {
		return -1, -1
	}

	upper := endBlock + int(s.SamplingDegree) - 1
	fmt.Println("Total index blocks: ", s.TotalIndexBlocks)
	if upper >= int(s.TotalIndexBlocks) {
		upper = int(s.TotalIndexBlocks) - 1
	}

	return startBlock, upper
}

////////////////////////////////////////////////////////////
// MEMORY ESTIMATION
////////////////////////////////////////////////////////////

func (s *SummarySegment) EstimatedSize() int {
	size := 0

	size += binary.MaxVarintLen64 + len(s.MinKey)
	size += binary.MaxVarintLen64 + len(s.MaxKey)

	size += 4 // sampling
	size += 4 // total blocks
	size += 4 // entry count

	for _, e := range s.Entries {
		size += binary.MaxVarintLen64 + len(e.Key)
		size += 8
	}

	size += 4 // crc

	return size
}

////////////////////////////////////////////////////////////
// BUILD FROM INDEX SEGMENT
////////////////////////////////////////////////////////////

func BuildSummaryFromIndex(
	indexBlockOffsets []uint64,
	indexBlocks []*IndexBlock,
	samplingDegree uint32,
) *SummarySegment {

	summary := NewSummarySegment(samplingDegree)
	summary.TotalIndexBlocks = uint32(len(indexBlocks))

	if len(indexBlocks) == 0 {
		return summary
	}

	// MinKey
	firstBlock := indexBlocks[0]
	if len(firstBlock.Entries) > 0 {
		summary.MinKey = append([]byte{}, firstBlock.Entries[0].Key...)
	}

	// MaxKey
	lastBlock := indexBlocks[len(indexBlocks)-1]
	if len(lastBlock.Entries) > 0 {
		lastEntry := lastBlock.Entries[len(lastBlock.Entries)-1]
		summary.MaxKey = append([]byte{}, lastEntry.Key...)
	}

	for i := 0; i < len(indexBlocks); i += int(samplingDegree) {
		block := indexBlocks[i]
		if len(block.Entries) == 0 {
			continue
		}

		keyCopy := append([]byte{}, block.Entries[0].Key...)

		summary.Entries = append(summary.Entries, SummaryEntry{
			Key:              keyCopy,
			IndexBlockOffset: indexBlockOffsets[i],
		})
	}

	return summary
}

////////////////////////////////////////////////////////////
// FILE IO
////////////////////////////////////////////////////////////

func (s *SummarySegment) WriteToFile(file *os.File) (int, error) {
	data := s.Encode()
	return file.Write(data)
}

func ReadSummaryFromFile(file *os.File, offset uint64, size int) (*SummarySegment, error) {
	if _, err := file.Seek(int64(offset), 0); err != nil {
		return nil, err
	}

	data := make([]byte, size)
	if _, err := file.Read(data); err != nil {
		return nil, err
	}

	return DecodeSummarySegment(data)
}
