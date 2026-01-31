package wal

import (
	"encoding/binary"
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

// Fragment = FragmentHeader + PayLoad bytes
type Fragment struct {
	Header  FragmentHeader
	Payload []byte
}

var (
	ErrShortHeader     = errors.New("fragment: not enough bytes for header")
	ErrInvalidType     = errors.New("fragment: invalid fragment type")
	ErrSizeMismatch    = errors.New("fragment: header size does not match payload length")
	ErrCorrupted       = errors.New("fragment: crc mismatch (corrupted or partial write)")
	ErrPayloadTooLarge = errors.New("fragment: payload too large for uint16 size field")
)

// ComputeFragmentCRC computes CRC using (Type + Payload)
func ComputeFragmentCRC(t FragmentType, payload []byte) uint32 {
	// 1 byte type + payload
	buf := make([]byte, 1+len(payload))
	buf[0] = byte(t)
	copy(buf[1:], payload)
	return crc32.ChecksumIEEE(buf)
}

// NewFragment creates a new fragment and fills header (CRC/Size/Type/LogNumber)
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

// EncodeHeader writes header in dst
func EncodeHeader(h FragmentHeader, dst []byte) error {
	if len(dst) < FragmentHeaderSize {
		return ErrShortHeader
	}
	if !h.Type.Valid() {
		return ErrInvalidType
	}

	binary.BigEndian.PutUint32(dst[0:4], h.CRC)
	binary.BigEndian.PutUint16(dst[4:6], h.Size)
	dst[6] = byte(h.Type)
	binary.BigEndian.PutUint32(dst[7:11], h.LogNumber)

	return nil
}

// DecodeHeader reads header from src
func DecodeHeader(src []byte) (FragmentHeader, error) {
	if len(src) < FragmentHeaderSize {
		return FragmentHeader{}, ErrShortHeader
	}

	h := FragmentHeader{
		CRC:       binary.BigEndian.Uint32(src[0:4]),
		Size:      binary.BigEndian.Uint16(src[4:6]),
		Type:      FragmentType(src[6]),
		LogNumber: binary.BigEndian.Uint32(src[7:11]),
	}

	if !h.Type.Valid() {
		return FragmentHeader{}, ErrInvalidType
	}
	return h, nil
}

// Verifies: 1) payload size 2) CRC integrity
func (f *Fragment) Verify() error {
	if int(f.Header.Size) != len(f.Payload) {
		return ErrSizeMismatch
	}
	expected := ComputeFragmentCRC(f.Header.Type, f.Payload)
	if expected != f.Header.CRC {
		return ErrCorrupted
	}
	return nil
}
