package wal

import (
	"fmt"
	"path/filepath"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
)

type WAL struct {
	Dir           string
	ActiveSegment *Segment
	BlockSize     int
	MaxBlocks     int
	NextSegmentID uint64
	BM            *block.BlockManager
}

func OpenWAL(dir string, blockSize int, maxBlocks int) (*WAL, error) {
	if dir == "" {
		return nil, fmt.Errorf("wal dir is empty")
	}
	if blockSize < KEY_START+1 {
		return nil, fmt.Errorf("blockSize is smaller than minimum WAL fragment size")
	}
	if maxBlocks <= 0 {
		return nil, fmt.Errorf("maxBlocks must be >0")
	}
	err := block.EnsureDir(dir)
	if err != nil {
		return nil, err
	}
	bm := block.NewBlockManager(blockSize)

	w := &WAL{
		Dir:           dir,
		BlockSize:     blockSize,
		MaxBlocks:     maxBlocks,
		NextSegmentID: 1,
		BM:            bm,
	}

	firstPath := w.SegmentPath(w.NextSegmentID)
	seg, err := OpenSegment(w.NextSegmentID, firstPath, w.MaxBlocks, w.BM)
	if err != nil {
		return nil, err
	}
	w.ActiveSegment = seg
	w.NextSegmentID++

	return w, nil

}

func (w *WAL) SegmentPath(id uint64) string {
	filename := fmt.Sprintf("wal_%06d.log", id)
	return filepath.Join(w.Dir, filename)
}
