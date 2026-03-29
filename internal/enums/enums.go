package enums

type SegmentType int

const (
	SegmentData       SegmentType = 0
	SegmentFilter     SegmentType = 1
	SegmentIndex      SegmentType = 2
	SegmentSummary    SegmentType = 3
	SegmentMerkleTree SegmentType = 4
	SegmentMetadata   SegmentType = 5
	SegmentFooter     SegmentType = 6
	SegmentDictionary SegmentType = 7
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

type LSMCompaction byte

const (
	LeveledCompaction    LSMCompaction = 0
	SizeTieredCompaction LSMCompaction = 1
)

type MemTableType byte

const (
	HashMapMemTable  MemTableType = 0
	SkiplistMemTable MemTableType = 1
	BTreeMemTable    MemTableType = 2
	RBTreeMemTable   MemTableType = 3
	AVLTreeMemTable  MemTableType = 4
)

type MergeStructureType byte

const (
	Heap  MergeStructureType = 0
	WTree MergeStructureType = 1
)
