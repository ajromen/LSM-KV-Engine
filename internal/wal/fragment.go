package wal

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
	header  FragmentHeader
	Payload []byte
}
