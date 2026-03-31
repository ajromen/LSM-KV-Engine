package wal

import (
	"fmt"
	"os"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
)

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
	exists, err := FileExists(path)
	if err != nil {
		return nil, err
	}
	if exists {
		// to do
		return nil, fmt.Errorf("not implemented yet")
	} else {
		s := Segment{
			ID:                id,
			Path:              path,
			BlockSize:         bm.BlockSize(),
			MaxBlocks:         maxBlocks,
			CurrentBlockIndex: 0,
			CurrentBlock:      NewBlock(bm.BlockSize()),
			BM:                bm,
		}
		err := bm.EnsureSize(path, int64(bm.BlockSize()*maxBlocks))
		if err != nil {
			return nil, err
		}
		return &s, nil
	}
}

func (s *Segment) Append(r Record) error {

	return fmt.Errorf("Not implemented yet")
}

func (s *Segment) ReadBlock(index uint32) ([]byte, error) {
	bk := block.BlockKey{
		FilePath: s.Path,
		Offset:   index,
	}
	buff, err := s.BM.ReadNoCache(bk)
	if err != nil {
		return nil, err
	}
	return buff, nil
}

func (s *Segment) FlushCurrentBlock() error {
	if s.CurrentBlock == nil {
		return fmt.Errorf("current block is nil")
	}
	bk := block.BlockKey{
		FilePath: s.Path,
		Offset:   s.CurrentBlockIndex,
	}
	err := s.BM.WriteNoCache(bk, s.CurrentBlock.Data)
	if err != nil {
		return err
	}
	return nil
}

func (s *Segment) MoveToNextBlock() error {
	if s.CurrentBlockIndex+1 >= uint32(s.MaxBlocks) {
		return fmt.Errorf("segment is full")
	}
	err := s.FlushCurrentBlock()
	if err != nil {
		return err
	}
	s.CurrentBlockIndex++
	s.CurrentBlock = NewBlock(s.BlockSize)
	return nil
}

// Helper function, should be moved to block manager?
func FileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
