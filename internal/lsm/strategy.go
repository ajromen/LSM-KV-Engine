package lsm

import (
	"bytes"
	"fmt"
	"math"

	"github.com/ajromen/LSM-KV-Engine/internal/sstable"
)

type CompactionStrategy interface {
	Compact(sstables *sstable.SSTableManager) error
}

type LeveledCompaction struct {
	L1MaxBytes          int64
	LevelSizeMultiplier int
}

func (l LeveledCompaction) Compact(manager *sstable.SSTableManager) error {
	for i := 0; i < len(manager.Layers); i++ {
		size := manager.Layers[i].GetSize()
		print(size, " ", l.L1MaxBytes*int64(math.Pow(float64(l.LevelSizeMultiplier), float64(i))))
		if size < l.L1MaxBytes*int64(math.Pow(float64(l.LevelSizeMultiplier), float64(i))) {
			continue
		}
		pickedCurrent := l.pickFromCurrent(manager.Layers[i].SSTables)
		// there is no next layer
		if i+1 == len(manager.Layers) {
			err := manager.MoveSSTable(pickedCurrent, i+1)
			if err != nil {
				return err
			}
			break
		}
		pickedNextLayer := l.pickFromNext(pickedCurrent, manager.Layers[i+1].SSTables)
		fmt.Printf("Compacting %d SSTables at layer %d\n", len(pickedNextLayer)+1, i)
		err := manager.MergeSSTables(append(pickedNextLayer, pickedCurrent), i+1)
		if err != nil {
			return err
		}
	}
	return nil
}

func (l LeveledCompaction) pickFromCurrent(records []*sstable.SSTableReader) *sstable.SSTableReader {
	if len(records) == 0 {
		return nil
	}
	return records[0] // najstariji
}

func (l LeveledCompaction) pickFromNext(currRecord *sstable.SSTableReader, nextLayer []*sstable.SSTableReader) []*sstable.SSTableReader {
	minKey := currRecord.SummarySegment.MinKey
	maxKey := currRecord.SummarySegment.MaxKey

	overlapping := make([]*sstable.SSTableReader, 0)
	for _, sst := range nextLayer {
		if bytes.Compare(sst.SummarySegment.MaxKey, minKey) < 0 {
			continue
		}
		if bytes.Compare(sst.SummarySegment.MinKey, maxKey) > 0 {
			continue
		}
		overlapping = append(overlapping, sst)
	}
	return overlapping
}

type SizeTiredCompaction struct {
	MinMergeThreshold int
}

func (s SizeTiredCompaction) Compact(manager *sstable.SSTableManager) error {
	for i := 0; i < len(manager.Layers); i++ {
		if manager.Layers[i].Length() < s.MinMergeThreshold {
			continue
		}
		fmt.Printf("Compacting %d SSTables at layer %d\n", len(manager.Layers[i].SSTables), i)
		err := manager.MergeSSTables(manager.Layers[i].SSTables, i+1)
		if err != nil {
			return err
		}
	}
	return nil
}
