package wal

import "errors"

type Block struct {
	Data     []byte
	Size     int
	WritePos int
}

func NewBlock(size int) *Block {
	return &Block{
		Data:     make([]byte, size),
		Size:     size,
		WritePos: 0,
	}
}

func (b *Block) Remaining() int {
	return b.Size - b.WritePos
}

func (b *Block) Used() int {
	return b.WritePos
}

func (b *Block) IsFull() bool {
	return b.WritePos >= b.Size
}

func (b *Block) CanFit(n int) bool {
	return n <= b.Remaining()
}

func (b *Block) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	if b.IsFull() {
		return 0, errors.New("block is full")
	}

	n := copy(b.Data[b.WritePos:], data)
	b.WritePos += n
	return n, nil
}

func (b *Block) Reset() {
	for i := range b.Data {
		b.Data[i] = 0
	}
	b.WritePos = 0
}
