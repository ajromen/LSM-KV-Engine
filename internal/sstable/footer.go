package sstable

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/utils"
)

const (
	FooterSize  = 137
	MagicNumber = 0x53535442
)

type SegmentHandler struct {
	Offset uint64 // holds the real offset in file for each of segments of sstable (in case of multifileformat = 0)
	Size   uint32 // size of each segment
}

type Footer struct {
	FilterHandler          SegmentHandler // handler for filter segment
	IndexHandler           SegmentHandler // handler for index segment
	SummaryHandler         SegmentHandler // handler for summary segment
	MetaDataHandler        SegmentHandler // handler for metadatahandler
	DictionaryHandler      SegmentHandler
	NumDataBlocks          uint32        // number of data blocks in sstable
	BlockSize              uint64        // block size in sstable
	MinTimeStamp           utils.Uint128 // min timestamp in sstable
	MaxTimeStamp           utils.Uint128 // max timestamp in sstable
	MinKeyLength           uint32        // min keylength in sstable
	MaxKeyLength           uint32        // max keylength in sstable
	TotalRecords           uint64        // number of records in sstable
	RestartInterval        uint32        // restart interval
	EncodingType           byte          // encoding type - 0 - delta encoding / 1 - dict delta encoding
	CompressionType        byte          // type of compression = 0 always
	MergeIteratorStructure byte          // 0 - heap / 1 - winner-tree
	Version                byte          // version = 1 always
	Format                 byte          // format (0 - singlefile / 1 - multifile)
	MagicNumber            uint32        // SSTB in hex
	CRC                    uint32        // crc over the whole footer segment
}

func NewFooter(config config.SSTableConfig) *Footer {
	formatByte := byte(0)
	if config.Format == 1 {
		formatByte = byte(1)
	}
	return &Footer{
		CompressionType:        config.DataSegment.Compression,
		Version:                1,
		Format:                 formatByte,
		MagicNumber:            MagicNumber,
		RestartInterval:        uint32(config.DataSegment.RestartInterval),
		EncodingType:           1,
		MergeIteratorStructure: 1,
		BlockSize:              uint64(config.DataSegment.BlockSize),
	}
}

// ENCODES SLICE OF BYTES TO FOOTER

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

	binary.LittleEndian.PutUint64(buf[pos:], f.DictionaryHandler.Offset)
	pos += 8
	binary.LittleEndian.PutUint32(buf[pos:], f.DictionaryHandler.Size)
	pos += 4

	binary.LittleEndian.PutUint64(buf[pos:], f.BlockSize)
	pos += 8
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

	binary.LittleEndian.PutUint32(buf[pos:], f.RestartInterval)
	pos += 4

	buf[pos] = f.EncodingType
	pos++
	buf[pos] = f.CompressionType
	pos++
	buf[pos] = f.MergeIteratorStructure
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

// DECODES FOOTER TO SLICE OF BYTES

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

	f.DictionaryHandler.Offset = binary.LittleEndian.Uint64(buf[pos:])
	pos += 8
	f.DictionaryHandler.Size = binary.LittleEndian.Uint32(buf[pos:])
	pos += 4

	f.BlockSize = binary.LittleEndian.Uint64(buf[pos:])
	pos += 8
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

	f.RestartInterval = binary.LittleEndian.Uint32(buf[pos:])
	pos += 4

	f.EncodingType = buf[pos]
	pos++
	f.CompressionType = buf[pos]
	pos++
	f.MergeIteratorStructure = buf[pos]
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

func (f *Footer) Validate() error {
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
