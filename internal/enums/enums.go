package enums

type SegmentType int

const (
	SegmentData     SegmentType = 0
	SegmentFilter   SegmentType = 1
	SegmentIndex    SegmentType = 2
	SegmentSummary  SegmentType = 3
	SegmentMetadata SegmentType = 4
	SegmentFooter   SegmentType = 5
)

type SSTableCompression byte

const (
	CompressionNone   SSTableCompression = 0
	CompressionSnappy SSTableCompression = 1
	CompressionZSTD   SSTableCompression = 2
)

type SSTableFormat byte

const (
	FormatSingleFile SSTableFormat = 0
	FormatMultiFile  SSTableFormat = 1
)

type LSMCompression byte

const (
	LeveledCompaction    LSMCompression = 0
	SizeTieredCompaction LSMCompression = 1
)

type MemTableType byte

const (
	HashMapMemTable  MemTableType = 0
	SkiplistMemTable MemTableType = 1
	BTreeMemTable    MemTableType = 2
	RBTreeMemTable   MemTableType = 3
	AVLTreeMemTable  MemTableType = 4
)
