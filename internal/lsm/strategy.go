package compaction

type CompactionStrategy interface {
	Compact(layers []Layer) []Layer
}

type LeveledCompaction struct {
	LevelSizeMultiplier int
}

type SizeTiredCompaction struct {
	MinMergeThreshold int
}
