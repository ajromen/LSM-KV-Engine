package wal

import "github.com/ajromen/LSM-KV-Engine/internal/block"

type Segment struct {
	ID                uint64
	Path              string
	BlockSize         int
	MaxBlocks         int
	CurrentBlockIndex uint32
	CurrentBlock      *Block
	BM                *block.BlockManager
}

func OpenSegment(id uint64, path string, maxBlocks int, bm *block.BlockManager) (*Segment, error) {
	s := Segment{
		ID:                id,
		Path:              path,
		BlockSize:         bm.BlockSize(),
		MaxBlocks:         maxBlocks,
		CurrentBlockIndex: 0,
		CurrentBlock:      NewBlock(bm.BlockSize()),
		BM:                bm,
	}
	return &s, nil
}
