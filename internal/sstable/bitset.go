package sstable

type BitSet struct {
	bytes []byte
	bits  int // number of bits that are used
}

func NewBitSet(capacity int) *BitSet {
	numBytes := (capacity + 7) / 8
	if numBytes == 0 {
		numBytes = 1
	}
	return &BitSet{
		bytes: make([]byte, numBytes),
		bits:  capacity,
	}
}

func (b *BitSet) Set(idx int) {
	byteIdx := idx / 8
	bitIdx := uint(idx % 8)
	for len(b.bytes) <= byteIdx {
		b.bytes = append(b.bytes, 0)
	}
	b.bytes[byteIdx] |= 1 << bitIdx
	if idx >= b.bits {
		b.bits = idx + 1
	}
}

func (b *BitSet) Get(idx int) bool {
	byteIdx := idx / 8
	if byteIdx >= len(b.bytes) {
		return false
	}
	bitIdx := uint(idx % 8)
	return (b.bytes[byteIdx] & (1 << bitIdx)) != 0
}

func (b *BitSet) Encode() []byte {
	if b.bits == 0 {
		return []byte{}
	}
	return append([]byte(nil), b.bytes...)
}

func DecodeBitSet(data []byte) *BitSet {
	return &BitSet{
		bytes: append([]byte(nil), data...),
		bits:  len(data) * 8,
	}
}
