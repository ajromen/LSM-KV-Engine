package sstable

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
)

const (
	FooterSize  = 106
	MagicNumber = 0x53535442
)

type SegmentHandler struct {
	Offset uint64 // holds the real offset in file for each of segments of sstable (in case of multifileformat = 0)
	Size   uint32 // size of each segment
}

type Footer struct {
	FilterHandler        SegmentHandler      // handler for filter segment
	IndexHandler         SegmentHandler      // handler for index segment
	TTLIndexHandler      SegmentHandler      // handler for index segment
	RangeDelIndexHandler SegmentHandler      // handler for range deletion segment
	SummaryHandler       SegmentHandler      // handler for summary segment
	MerkleHandler        SegmentHandler      // handler for merkle tree segment
	MetaDataHandler      SegmentHandler      // handler for metadata segment
	DictionaryHandler    SegmentHandler      // handler for dictionary segment
	Version              byte                // version = 1 always
	Format               enums.SSTableFormat // format (0 - singlefile / 1 - multifile)
	MagicNumber          uint32              // SSTB in hex
	CRC                  uint32              // crc over the whole footer segment
}

func NewFooter() *Footer {
	cfg := config.GetSettings().SSTable
	return &Footer{
		Version:     1,
		Format:      cfg.Format,
		MagicNumber: MagicNumber,
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

	binary.LittleEndian.PutUint64(buf[pos:], f.TTLIndexHandler.Offset)
	pos += 8
	binary.LittleEndian.PutUint32(buf[pos:], f.TTLIndexHandler.Size)
	pos += 4

	binary.LittleEndian.PutUint64(buf[pos:], f.RangeDelIndexHandler.Offset)
	pos += 8
	binary.LittleEndian.PutUint32(buf[pos:], f.RangeDelIndexHandler.Size)
	pos += 4

	binary.LittleEndian.PutUint64(buf[pos:], f.SummaryHandler.Offset)
	pos += 8
	binary.LittleEndian.PutUint32(buf[pos:], f.SummaryHandler.Size)
	pos += 4

	binary.LittleEndian.PutUint64(buf[pos:], f.MerkleHandler.Offset)
	pos += 8
	binary.LittleEndian.PutUint32(buf[pos:], f.MerkleHandler.Size)
	pos += 4

	binary.LittleEndian.PutUint64(buf[pos:], f.MetaDataHandler.Offset)
	pos += 8
	binary.LittleEndian.PutUint32(buf[pos:], f.MetaDataHandler.Size)
	pos += 4

	binary.LittleEndian.PutUint64(buf[pos:], f.DictionaryHandler.Offset)
	pos += 8
	binary.LittleEndian.PutUint32(buf[pos:], f.DictionaryHandler.Size)
	pos += 4

	buf[pos] = f.Version
	pos++
	buf[pos] = byte(f.Format)
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

	f.TTLIndexHandler.Offset = binary.LittleEndian.Uint64(buf[pos:])
	pos += 8
	f.TTLIndexHandler.Size = binary.LittleEndian.Uint32(buf[pos:])
	pos += 4

	f.RangeDelIndexHandler.Offset = binary.LittleEndian.Uint64(buf[pos:])
	pos += 8
	f.RangeDelIndexHandler.Size = binary.LittleEndian.Uint32(buf[pos:])
	pos += 4

	f.SummaryHandler.Offset = binary.LittleEndian.Uint64(buf[pos:])
	pos += 8
	f.SummaryHandler.Size = binary.LittleEndian.Uint32(buf[pos:])
	pos += 4

	f.MerkleHandler.Offset = binary.LittleEndian.Uint64(buf[pos:])
	pos += 8
	f.MerkleHandler.Size = binary.LittleEndian.Uint32(buf[pos:])
	pos += 4

	f.MetaDataHandler.Offset = binary.LittleEndian.Uint64(buf[pos:])
	pos += 8
	f.MetaDataHandler.Size = binary.LittleEndian.Uint32(buf[pos:])
	pos += 4

	f.DictionaryHandler.Offset = binary.LittleEndian.Uint64(buf[pos:])
	pos += 8
	f.DictionaryHandler.Size = binary.LittleEndian.Uint32(buf[pos:])
	pos += 4

	f.Version = buf[pos]
	pos++
	f.Format = enums.SSTableFormat(buf[pos])
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
	_, _, err := storage.WriteSegment(enums.SegmentFooter, encoded)
	return err
}

func (f *Footer) ReadFromStorage(storage SegmentStorage, offset uint64) error {
	data, err := storage.ReadSegment(enums.SegmentFooter, offset, FooterSize)
	if err != nil {
		return err
	}

	return f.Decode(data)
}
