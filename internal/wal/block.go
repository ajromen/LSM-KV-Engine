package wal

import "errors"

var (
	ErrNotEnoughSpace = errors.New("block: not enough space")
	ErrOutOfBounds    = errors.New("block: read out of bounds")
)

type Block struct {
	buf []byte
	pos int
}

func NewBlock(blockSize int) *Block {
	return &Block{
		buf: make([]byte, blockSize),
		pos: 0,
	}
}

func (b *Block) Size() int {
	return len(b.buf)
}

func (b *Block) Pos() int {
	return b.pos
}

func (b *Block) Remaining() int {
	return len(b.buf) - b.pos
}

func (b *Block) Reset() {
	for i := range b.buf {
		b.buf[i] = 0
	}
	b.pos = 0
}

func (b *Block) Bytes() []byte {
	return b.buf
}

// PadToEnd fills remaining space with zeroes
func (b *Block) PadToEnd() {
	for b.pos < len(b.buf) {
		b.buf[b.pos] = 0
		b.pos++
	}
}

// WriteBytes writes p to block
func (b *Block) WriteBytes(p []byte) error {
	if len(p) > b.Remaining() {
		return ErrNotEnoughSpace
	}
	copy(b.buf[b.pos:], p)
	b.pos += len(p)
	return nil
}

// ReadBytes reads n bytes from current pos and shifts pos
func (b *Block) ReadBytes(n int) ([]byte, error) {
	if n < 0 || n > b.Remaining() {
		return nil, ErrOutOfBounds
	}
	out := b.buf[b.pos : b.pos+n]
	b.pos += n
	return out, nil
}

// PeekBytes reads n bytes without shifting pos
func (b *Block) PeekBytes(n int) ([]byte, error) {
	if n < 0 || n > b.Remaining() {
		return nil, ErrOutOfBounds
	}
	return b.buf[b.pos : b.pos+n], nil
}

// Skip moves pos for n spaces (for skipping padding)
func (b *Block) Skip(n int) error {
	if n < 0 || n > b.Remaining() {
		return ErrOutOfBounds
	}
	b.pos += n
	return nil
}
