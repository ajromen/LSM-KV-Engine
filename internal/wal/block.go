package wal

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
