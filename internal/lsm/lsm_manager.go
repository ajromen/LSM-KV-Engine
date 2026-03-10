package lsm

import (
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/sstable"
)

type LSMLayer struct {
	Index uint
	Table []*sstable.SSTableReader
}

type LSMTree struct {
	Layers []LSMLayer

	CompactionType     string
	compactionCallback func()
	MaxLayers          int
}

func NewLSMTree(cfg *config.LSMTreeConfig) LSMTree {
	lsm := LSMTree{}
	lsm.MaxLayers = cfg.MaxLevels
	lsm.CompactionType = cfg.CompactionAlgorithm
	return lsm
}
