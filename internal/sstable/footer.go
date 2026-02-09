package sstable

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/ajromen/LSM-KV-Engine/internal/utils"
)

const (
	FooterSize        = 94
	MagicNumber       = 0x53535442 // SSTB IN HEX
	MagicNumberOffset = FooterSize - 8
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
	// samo ovako nastavi za sve i odradi checksum nad svime na kraju
	return buf
}

func (f *Footer) Decode(buf []byte) error {
	pos := 0
	f.FilterHandler.Offset = binary.LittleEndian.Uint64(buf[pos:])
	pos += 8
	f.FilterHandler.Size = binary.LittleEndian.Uint32(buf[pos:])
	pos += 4
	// isto samo nastavljaj dalje i verifikuj magic number i checksum
	return nil
}

func (f *Footer) WriteToFile(file *os.File) error {
	encoded := f.Encode()
	_, err := file.Write(encoded)
	if err != nil {
		return err
	}
	return nil
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
	err = footer.Decode(buf)
	if err != nil {
		return nil, err
	}
	return footer, nil
}

// zavrsi znaci samo Encode i Decode i odradi Validate

// TODO : ENCODE/DECODE/WRITEFOOTERTOFILE/READFOOTERFROMFILE/VALIDATE/VISUALIZE -> igraj se strahinja sa fajlovima malo
/*
├─────────────────────────────────────────────────┤
│                 FOOTER                          │
│  - Filter Block Offset (8 bytes)                │
│  - Filter Block Size (4 bytes)                  │
│  - Index Block Offset (8 bytes)                 │
│  - Index Block Size (4 bytes)                   │
│  - Summary Block Offset (8 bytes)               │
│  - Summary Block Size (4 bytes)                 │
│  - Metadata Block Offset (8 bytes)              │
│  - Metadata Block Size (4 bytes)                │
│  - Number of Data Blocks (4 bytes)              │
│  - Min Timestamp (16 bytes - uint128)           │
│  - Max Timestamp (16 bytes - uint128)           │
│  - Min Key Length (2 bytes)                     │
│  - Max Key Length (2 bytes)                     │
│  - Total Records (8 bytes)                      │
│  - Compression Type (1 byte)                    │
│  - Version (1 byte)                             │
│  - Magic Number (4 bytes) "SSTB"                │
│  - Footer CRC32 (4 bytes)                       │
│                                                 │
│              TOTAL: 94 bytes                    │
└─────────────────────────────────────────────────┘
*/
