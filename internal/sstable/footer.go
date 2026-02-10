package sstable

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
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
	Format          byte
	MagicNumber     uint32
	CRC             uint32
}

func NewFooter(config config.SSTableConfig) *Footer {
	formatByte := byte(0)
	if config.Format == 1 {
		formatByte = byte(1)
	}
	return &Footer{
		CompressionType: config.DataSegment.Compression,
		Version:         1,
		Format:          formatByte,
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
	buf[pos] = f.Format
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
	f.Format = buf[pos]
	pos++

	f.MagicNumber = binary.LittleEndian.Uint32(buf[pos:])
	pos += 4

	if f.MagicNumber != MagicNumber {
		return fmt.Errorf("invalid magic number: expected 0x%X, got 0x%X", MagicNumber, f.MagicNumber)
	}

	expectedCRC := binary.LittleEndian.Uint32(buf[pos:])
	actualCRC := crc32.ChecksumIEEE(buf[:pos])

	if expectedCRC != actualCRC {
		return errors.New("footer CRC mismatch")
	}

	f.CRC = expectedCRC

	return nil
}

func (f *Footer) Validate(buf []byte) error {
	if f.MagicNumber != MagicNumber {
		return fmt.Errorf("invalid magic number: %x", f.MagicNumber)
	}

	if f.Version != 1 {
		return fmt.Errorf("unsupported footer version: %d", f.Version)
	}

	if f.Format > 1 {
		return fmt.Errorf("invalid format: %d", f.Format)
	}

	return nil
}

func (f *Footer) WriteToStorage(storage SegmentStorage) error {
	encoded := f.Encode()
	_, _, err := storage.WriteSegment(config.SegmentFooter, encoded)
	return err
}

func (f *Footer) ReadFromStorage(storage SegmentStorage, offset uint64) error {
	data, err := storage.ReadSegment(config.SegmentFooter, offset, FooterSize)
	if err != nil {
		return err
	}

	return f.Decode(data)
}

func (f *Footer) WriteToFile(file *os.File) error {
	buf := f.Encode()
	_, err := file.Write(buf)
	return err
}

func (f *Footer) ReadFromFile(file *os.File) error {
	_, err := file.Seek(-FooterSize, io.SeekEnd)
	if err != nil {
		return err
	}

	buf := make([]byte, FooterSize)
	n, err := file.Read(buf)
	if err != nil {
		return err
	}

	if n != FooterSize {
		return fmt.Errorf("expected %d bytes, got %d", FooterSize, n)
	}
	return f.Decode(buf)
}

func ReadFooterFromFile(filePath string) (*Footer, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	footer := &Footer{}
	err = footer.ReadFromFile(file)
	if err == nil && footer.Format == 0 {
		return footer, nil
	}
	footerFile, err := os.Open(filePath + ".footer")
	if err != nil {
		return nil, fmt.Errorf("failed to read footer: %w", err)
	}
	defer footerFile.Close()
	buf := make([]byte, FooterSize)
	n, err := footerFile.Read(buf)
	if err != nil {
		return nil, err
	}
	if n != FooterSize {
		return nil, fmt.Errorf("invalid footer size: got %d bytes", n)
	}
	footer = &Footer{}
	if err := footer.Decode(buf); err != nil {
		return nil, err
	}
	return footer, nil
}
