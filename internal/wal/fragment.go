package wal

import (
	"errors"
	"hash/crc32"
)

type FragmentType uint8

// A single record may be in one or muliple blocks, depending on its size
// Fragment types: FULL, FIRST, MIDDLE, LAST
const (
	FragFull   FragmentType = 1
	FragFirst  FragmentType = 2
	FragMiddle FragmentType = 3
	FragLast   FragmentType = 4
)

func (t FragmentType) Valid() bool {
	return t == FragFull || t == FragFirst || t == FragMiddle || t == FragLast
}

type FragmentHeader struct {
	CRC       uint32
	Size      uint16
	Type      FragmentType
	LogNumber uint32
}

const (
	fragCRCLen       = 4
	fragSizeLen      = 2
	fragTypeLen      = 1
	fragLogNumberLen = 4

	FragmentHeaderSize = fragCRCLen + fragSizeLen + fragTypeLen + fragLogNumberLen
)

// Fragment = FragmentHeader + Record
type Fragment struct {
	Header  FragmentHeader
	Payload []byte
}

var (
	ErrInvalidType     = errors.New("fragment: invalid fragment type")
	ErrPayloadTooLarge = errors.New("fragment: payload too large for uint16 size field")
)

// ComputeFragmentCRC computes CRC using (Type + Payload).
func ComputeFragmentCRC(t FragmentType, payload []byte) uint32 {
	// 1 byte type + payload
	buf := make([]byte, 1+len(payload))
	buf[0] = byte(t)
	copy(buf[1:], payload)
	return crc32.ChecksumIEEE(buf)
}

// NewFragment creates a new fragment and fills header (CRC/Size/Type/LogNumber).
func NewFragment(t FragmentType, logNumber uint32, payload []byte) (*Fragment, error) {
	if !t.Valid() {
		return nil, ErrInvalidType
	}
	if len(payload) > int(^uint16(0)) { // > 65535
		return nil, ErrPayloadTooLarge
	}

	h := FragmentHeader{
		Size:      uint16(len(payload)),
		Type:      t,
		LogNumber: logNumber,
	}
	h.CRC = ComputeFragmentCRC(t, payload)

	return &Fragment{Header: h, Payload: payload}, nil
}
