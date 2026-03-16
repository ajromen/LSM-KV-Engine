package lsm

import (
	"github.com/ajromen/LSM-KV-Engine/internal/sstable"
)

type CompactionStrategy interface {
	Compact(sstables *sstable.SSTableManager) error
}

type LeveledCompaction struct {
	LevelSizeMultiplier int
}

func (l LeveledCompaction) Compact(sstables *sstable.SSTableManager) error {
	panic("implement me")
}

type SizeTiredCompaction struct {
	MinMergeThreshold int
}

func (s SizeTiredCompaction) Compact(manager *sstable.SSTableManager) error {
	for i, layer := range manager.Layers {
		if layer.Length() > s.MinMergeThreshold {
			err := manager.MergeSSTables(layer.SSTables, uint64(i+1))
			if err != nil {
				return err
			}
		}
	}
	return nil
}
