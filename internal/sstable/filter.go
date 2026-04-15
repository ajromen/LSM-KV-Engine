package sstable

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"

	"github.com/ajromen/LSM-KV-Engine/internal/probabilistics"
)

type FilterSegment struct {
	filter *probabilistics.BloomFilter
}

func NewFilterSegment(filter *probabilistics.BloomFilter) *FilterSegment {
	return &FilterSegment{
		filter: filter,
	}
}

func (segment *FilterSegment) Filter() *probabilistics.BloomFilter {
	return segment.filter
}

////////////////////////////////////////////////////////////
// ENCODE / DECODE
////////////////////////////////////////////////////////////

// Format:
// u32   - bloom length
// []byte - bloom serialized
// u32   - CRC (over everything above)

func (segment *FilterSegment) Encode() ([]byte, error) {
	if segment == nil || segment.filter == nil {
		return nil, errors.New("nil filter segment")
	}

	bloomBytes, err := segment.filter.ToBytes()
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)

	// bloom length
	if err := binary.Write(buf, binary.LittleEndian, uint32(len(bloomBytes))); err != nil {
		return nil, err
	}

	// bloom data
	if _, err := buf.Write(bloomBytes); err != nil {
		return nil, err
	}

	// CRC
	crc := crc32.ChecksumIEEE(buf.Bytes())
	if err := binary.Write(buf, binary.LittleEndian, crc); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func DecodeFilterSegment(data []byte) (*FilterSegment, error) {
	if len(data) < 8 {
		return nil, errors.New("invalid filter segment size")
	}

	crcPos := len(data) - 4
	expectedCRC := binary.LittleEndian.Uint32(data[crcPos:])
	actualCRC := crc32.ChecksumIEEE(data[:crcPos])

	if expectedCRC != actualCRC {
		return nil, errors.New("filter CRC mismatch")
	}

	reader := bytes.NewReader(data)

	var bloomLen uint32
	if err := binary.Read(reader, binary.LittleEndian, &bloomLen); err != nil {
		return nil, err
	}

	bloomBytes := make([]byte, bloomLen)
	if _, err := reader.Read(bloomBytes); err != nil {
		return nil, err
	}

	bloom := &probabilistics.BloomFilter{}
	if err := bloom.FromBytes(bloomBytes); err != nil {
		return nil, err
	}

	return &FilterSegment{
		filter: bloom,
	}, nil
}
