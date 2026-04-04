package wal

import (
	"errors"
	"fmt"
	"os"
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

func OpenWAL(dir string, blockSize int, maxBlocks int) (*WAL, error) { //always opens from scratch, next segment id always 2, no matter how many files are already here
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

func (w *WAL) Append(r Record) error {
	if w == nil {
		return fmt.Errorf("wal is nil")
	}
	if w.ActiveSegment == nil {
		return fmt.Errorf("active segment is nil")
	}
	err := w.ActiveSegment.Append(r)
	if err == nil {
		return nil
	}
	if !w.ActiveSegment.IsFull() {
		return err
	}

	err = w.RotateSegment()
	if err != nil {
		return err
	}

	return w.ActiveSegment.Append(r)
}

func (w *WAL) RotateSegment() error {
	if w == nil {
		return fmt.Errorf("wal is nil")
	}
	if w.ActiveSegment == nil {
		return fmt.Errorf("active segment is nil")
	}
	err := w.ActiveSegment.Sync()
	if err != nil {
		return err
	}

	newPath := w.SegmentPath(w.NextSegmentID)
	seg, err := OpenSegment(w.NextSegmentID, newPath, w.MaxBlocks, w.BM)
	if err != nil {
		return err
	}
	w.ActiveSegment = seg
	w.NextSegmentID++
	return nil
}

func (w *WAL) Sync() error {
	if w == nil {
		return fmt.Errorf("wal is nil")
	}
	if w.ActiveSegment == nil {
		return fmt.Errorf("active segment is nil")
	}
	return w.ActiveSegment.Sync()
}

func (w *WAL) PrintAll() error { //func for debugging
	id := uint(1)
	for {
		path := w.SegmentPath(uint64(id))
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			break
		}
		s, err := OpenSegment(uint64(id), w.SegmentPath(uint64(id)), w.MaxBlocks, w.BM)
		if err != nil {
			break
		}
		bindex := 0
		fmt.Println("Segment", id)
		for {
			block, err := s.ReadBlock(uint32(bindex))
			if err != nil {
				break
			}
			fmt.Println(block)
			bindex++
		}
		id++

	}
	return nil
}

func (w *WAL) SegmentPath(id uint64) string {
	filename := fmt.Sprintf("wal_%06d.log", id)
	return filepath.Join(w.Dir, filename)
}
