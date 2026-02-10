package sstable

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"

	"github.com/ajromen/LSM-KV-Engine/internal/utils"
)

const (
	FooterSize  = 110
	MagicNumber = 0x53535442
)

type SegmentHandler struct {
	Offset uint64
	Size   uint32
}

type Footer struct {
	FilterHandler   SegmentHandler
	IndexHandler    SegmentHandler
	SummaryHandler  SegmentHandler
	MetaDataHandler SegmentHandler
	NumDataBlocks   uint32
	MinTimeStamp    utils.Uint128
	MaxTimeStamp    utils.Uint128
	MinKeyLength    uint32
	MaxKeyLength    uint32
	TotalRecords    uint64
	CompressionType byte
	Version         byte
	MagicNumber     uint32
	CRC             uint32
}

func NewFooter(compressionType byte) *Footer {
	return &Footer{
		CompressionType: compressionType,
		Version:         1,
		MagicNumber:     MagicNumber,
	}
}

func (f *Footer) Encode() []byte {
	buf := make([]byte, FooterSize)
	pos := 0

	binary.LittleEndian.PutUint64(buf[pos:], f.FilterHandler.Offset)
	pos += 8
	binary.LittleEndian.PutUint32(buf[pos:], f.FilterHandler.Size)
	pos += 4

	binary.LittleEndian.PutUint64(buf[pos:], f.IndexHandler.Offset)
	pos += 8
	binary.LittleEndian.PutUint32(buf[pos:], f.IndexHandler.Size)
	pos += 4

	binary.LittleEndian.PutUint64(buf[pos:], f.SummaryHandler.Offset)
	pos += 8
	binary.LittleEndian.PutUint32(buf[pos:], f.SummaryHandler.Size)
	pos += 4

	binary.LittleEndian.PutUint64(buf[pos:], f.MetaDataHandler.Offset)
	pos += 8
	binary.LittleEndian.PutUint32(buf[pos:], f.MetaDataHandler.Size)
	pos += 4

	binary.LittleEndian.PutUint32(buf[pos:], f.NumDataBlocks)
	pos += 4

	binary.LittleEndian.PutUint64(buf[pos:], f.MinTimeStamp.Low)
	binary.LittleEndian.PutUint64(buf[pos+8:], f.MinTimeStamp.High)
	pos += 16

	binary.LittleEndian.PutUint64(buf[pos:], f.MaxTimeStamp.Low)
	binary.LittleEndian.PutUint64(buf[pos+8:], f.MaxTimeStamp.High)
	pos += 16

	binary.LittleEndian.PutUint32(buf[pos:], f.MinKeyLength)
	pos += 4
	binary.LittleEndian.PutUint32(buf[pos:], f.MaxKeyLength)
	pos += 4

	binary.LittleEndian.PutUint64(buf[pos:], f.TotalRecords)
	pos += 8

	buf[pos] = f.CompressionType
	pos++
	buf[pos] = f.Version
	pos++

	binary.LittleEndian.PutUint32(buf[pos:], f.MagicNumber)
	pos += 4

	crc := crc32.ChecksumIEEE(buf[:pos])
	binary.LittleEndian.PutUint32(buf[pos:], crc)

	return buf
}

func (f *Footer) Decode(buf []byte) error {
	if len(buf) != FooterSize {
		return fmt.Errorf("invalid footer size: expected %d, got %d", FooterSize, len(buf))
	}

	pos := 0

	f.FilterHandler.Offset = binary.LittleEndian.Uint64(buf[pos:])
	pos += 8
	f.FilterHandler.Size = binary.LittleEndian.Uint32(buf[pos:])
	pos += 4

	f.IndexHandler.Offset = binary.LittleEndian.Uint64(buf[pos:])
	pos += 8
	f.IndexHandler.Size = binary.LittleEndian.Uint32(buf[pos:])
	pos += 4

	f.SummaryHandler.Offset = binary.LittleEndian.Uint64(buf[pos:])
	pos += 8
	f.SummaryHandler.Size = binary.LittleEndian.Uint32(buf[pos:])
	pos += 4

	f.MetaDataHandler.Offset = binary.LittleEndian.Uint64(buf[pos:])
	pos += 8
	f.MetaDataHandler.Size = binary.LittleEndian.Uint32(buf[pos:])
	pos += 4

	f.NumDataBlocks = binary.LittleEndian.Uint32(buf[pos:])
	pos += 4

	f.MinTimeStamp.Low = binary.LittleEndian.Uint64(buf[pos:])
	f.MinTimeStamp.High = binary.LittleEndian.Uint64(buf[pos+8:])
	pos += 16

	f.MaxTimeStamp.Low = binary.LittleEndian.Uint64(buf[pos:])
	f.MaxTimeStamp.High = binary.LittleEndian.Uint64(buf[pos+8:])
	pos += 16

	f.MinKeyLength = binary.LittleEndian.Uint32(buf[pos:])
	pos += 4
	f.MaxKeyLength = binary.LittleEndian.Uint32(buf[pos:])
	pos += 4

	f.TotalRecords = binary.LittleEndian.Uint64(buf[pos:])
	pos += 8

	f.CompressionType = buf[pos]
	pos++
	f.Version = buf[pos]
	pos++

	f.MagicNumber = binary.LittleEndian.Uint32(buf[pos:])
	pos += 4

	f.CRC = binary.LittleEndian.Uint32(buf[pos:])

	return f.Validate(buf)
}

func (f *Footer) Validate(buf []byte) error {
	if f.MagicNumber != MagicNumber {
		return fmt.Errorf("invalid magic number: %x", f.MagicNumber)
	}

	if f.Version != 1 {
		return fmt.Errorf("unsupported footer version: %d", f.Version)
	}

	calculated := crc32.ChecksumIEEE(buf[:FooterSize-4])
	if calculated != f.CRC {
		return fmt.Errorf("footer checksum mismatch")
	}

	return nil
}

func (f *Footer) WriteToFile(file *os.File) error {
	buf := f.Encode()
	_, err := file.Write(buf)
	return err
}

func (f *Footer) ReadFromFile(file *os.File) (*Footer, error) {
	_, err := file.Seek(-FooterSize, io.SeekEnd)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, FooterSize)
	n, err := file.Read(buf)
	if err != nil {
		return nil, err
	}

	if n != FooterSize {
		return nil, fmt.Errorf("expected %d bytes, got %d", FooterSize, n)
	}

	footer := &Footer{}
	if err := footer.Decode(buf); err != nil {
		return nil, err
	}

	return footer, nil
}
