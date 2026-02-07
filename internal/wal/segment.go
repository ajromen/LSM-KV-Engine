package wal

import "github.com/ajromen/LSM-KV-Engine/internal/block"

type Segment struct {
	filename string
	id       uint64
	bm       *block.BlockManager
}

func NewSegment(filename string, id uint64, bm *block.BlockManager) *Segment {
	return &Segment{
		filename: filename,
		id:       id,
		bm:       bm,
	}
}

func (s *Segment) WriteBlock(blockIndex uint32, blockBytes []byte) error {
	key := block.NewBlockKey(s.filename, blockIndex)
	return s.bm.Write(key, blockBytes)
}

func (s *Segment) ReadBlock(blockIndex uint32) ([]byte, error) {
	key := block.NewBlockKey(s.filename, blockIndex)
	return s.bm.Read(key)
}
