package sstable

import "github.com/ajromen/LSM-KV-Engine/internal/utils"

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
	DataHandler     SegmentHandler
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
