package compaction

import "github.com/ajromen/LSM-KV-Engine/internal/sstable"

type CompactionStrategy interface {
	Compact(layers []*sstable.Layer) []*sstable.Layer
	ShouldCompact(layers []*sstable.Layer) bool
}

type LeveledCompaction struct {
	LevelSizeMultiplier int
}

func (l LeveledCompaction) Compact(layers []*sstable.Layer) []*sstable.Layer {
	//TODO implement me
	panic("implement me")
}

func (l LeveledCompaction) ShouldCompact(layers []*sstable.Layer) bool {
	//TODO implement me
	panic("implement me")
}

type SizeTiredCompaction struct {
	MinMergeThreshold int
}

func (s SizeTiredCompaction) Compact(layers []*sstable.Layer) []*sstable.Layer {
	//TODO implement me
	panic("implement me")
}

func (s SizeTiredCompaction) ShouldCompact(layers []*sstable.Layer) bool {
	//TODO implement me
	panic("implement me")
}
