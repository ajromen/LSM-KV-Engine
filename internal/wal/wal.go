package wal

import "github.com/ajromen/LSM-KV-Engine/internal/block"

type WAL struct {
	Dir           string
	ActiveSegment *Segment
	BlockSize     int
	MaxBlock      int
	NextSegmentID uint64
	BM            *block.BlockManager
}
