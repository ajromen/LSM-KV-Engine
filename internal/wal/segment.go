package wal

import "github.com/ajromen/LSM-KV-Engine/internal/block"

type Segment struct {
	filename string
	id       uint64
	bm       *block.BlockManager
}

func NewSegment(filename string, id uint64, bm *block.BlockManager) *Segment {
	return &Segment{filename: filename, id: id, bm: bm}
}

func (s *Segment) Path() string { return s.filename }

func (s *Segment) WriteBlock(blockIndex uint32, blockBytes []byte) error {
	key := block.NewBlockKey(s.filename, blockIndex)
	return s.bm.Write(key, blockBytes)
}

func (s *Segment) ReadBlock(blockIndex uint32) ([]byte, error) {
	key := block.NewBlockKey(s.filename, blockIndex)
	return s.bm.Read(key)
}

// Sync segment file to disc (fsync)
func (s *Segment) Sync() error {
	return s.bm.SyncFile(s.filename)
}

// EnsureFixedSize preallocates segment with fixed size (bytes)
func (s *Segment) EnsureFixedSize(sizeBytes int64) error {
	return s.bm.EnsureSize(s.filename, sizeBytes)
}
