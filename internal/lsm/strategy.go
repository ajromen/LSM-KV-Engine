package lsm

import (
	"bytes"
	"fmt"
	"math"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/sstable"
)

type CompactionStrategy interface {
	Compact(sstables *sstable.SSTableManager) error
}

type LeveledCompaction struct {
	MaxHeight           int
	L1MaxBytes          int64
	LevelSizeMultiplier int
}

func (l LeveledCompaction) Compact(manager *sstable.SSTableManager) error {
	for i := 0; i < len(manager.Layers); i++ {
		layer := manager.Layers[i]
		if i == 0 {
			if layer.Length() < 4 {
				continue
			}
			if i+1 >= l.MaxHeight {
				return manager.MergeSSTables(layer.SSTables, i, true)
			}
			targetLayer := 1
			for len(manager.Layers) <= targetLayer {
				manager.Layers = append(manager.Layers, newLayer())
			}

			minKey, maxKey := l.l0Range(layer.SSTables)
			overlapping := l.overlappingInLayer(minKey, maxKey, manager.Layers[targetLayer].SSTables)
			toMerge := append(overlapping, layer.SSTables...)
			if config.GetSettings().Debug {
				fmt.Printf("Compacting L0 (%d files) into L1\n", len(layer.SSTables))
			}
			if err := manager.MergeSSTables(toMerge, targetLayer, false); err != nil {
				return err
			}
			continue
		}

		maxSize := l.L1MaxBytes * int64(math.Pow(float64(l.LevelSizeMultiplier), float64(i-1)))
		if layer.GetSize() < maxSize {
			continue
		}

		isLastLayer := i == l.MaxHeight-1
		if isLastLayer {
			if layer.Length() > 1 {
				return manager.MergeSSTables(layer.SSTables, i, true)
			}
			continue
		}

		picked := l.pickFromCurrent(layer.SSTables)
		if picked == nil {
			continue
		}
		targetLayer := min(i+1, l.MaxHeight-1)
		for len(manager.Layers) <= targetLayer {
			manager.Layers = append(manager.Layers, newLayer())
		}

		minK := picked.Metadata.GetBytes(sstable.FieldMinKey)
		maxK := picked.Metadata.GetBytes(sstable.FieldMaxKey)
		overlapping := l.overlappingInLayer(minK, maxK, manager.Layers[targetLayer].SSTables)

		if config.GetSettings().Debug {
			fmt.Printf("Compacting L%d → L%d (%d overlapping)\n", i, targetLayer, len(overlapping)+1)
		}

		if len(overlapping) == 0 {
			if err := manager.MoveSSTable(picked, targetLayer); err != nil {
				return err
			}
		} else {
			toMerge := append(overlapping, picked)
			if err := manager.MergeSSTables(toMerge, targetLayer, false); err != nil {
				return err
			}
		}
	}
	return nil
}

func (l LeveledCompaction) l0Range(tables []*sstable.SSTableReader) ([]byte, []byte) {
	var minKey, maxKey []byte
	for _, t := range tables {
		lo := t.Metadata.GetBytes(sstable.FieldMinKey)
		hi := t.Metadata.GetBytes(sstable.FieldMaxKey)
		if minKey == nil || bytes.Compare(lo, minKey) < 0 {
			minKey = lo
		}
		if maxKey == nil || bytes.Compare(hi, maxKey) > 0 {
			maxKey = hi
		}
	}
	return minKey, maxKey
}

func (l LeveledCompaction) overlappingInLayer(minKey, maxKey []byte, layer []*sstable.SSTableReader) []*sstable.SSTableReader {
	var out []*sstable.SSTableReader
	for _, sst := range layer {
		sstMin := sst.Metadata.GetBytes(sstable.FieldMinKey)
		sstMax := sst.Metadata.GetBytes(sstable.FieldMaxKey)
		if bytes.Compare(sstMax, minKey) < 0 {
			continue
		}
		if bytes.Compare(sstMin, maxKey) > 0 {
			continue
		}
		out = append(out, sst)
	}
	return out
}

func (l LeveledCompaction) pickFromCurrent(records []*sstable.SSTableReader) *sstable.SSTableReader {
	if len(records) == 0 {
		return nil
	}
	return records[0] // najstariji
}

func (l LeveledCompaction) pickFromNext(currRecord *sstable.SSTableReader, nextLayer []*sstable.SSTableReader) []*sstable.SSTableReader {
	minKey := currRecord.Metadata.GetBytes(sstable.FieldMinKey)
	maxKey := currRecord.Metadata.GetBytes(sstable.FieldMaxKey)

	overlapping := make([]*sstable.SSTableReader, 0)
	for _, sst := range nextLayer {
		sstMin := sst.Metadata.GetBytes(sstable.FieldMinKey)
		sstMax := sst.Metadata.GetBytes(sstable.FieldMaxKey)

		if bytes.Compare(sstMax, minKey) < 0 {
			continue
		}
		if bytes.Compare(sstMin, maxKey) > 0 {
			continue
		}
		overlapping = append(overlapping, sst)
	}
	return overlapping
}

type SizeTiredCompaction struct {
	MaxHeight         int
	MinMergeThreshold int
}

func (s SizeTiredCompaction) Compact(manager *sstable.SSTableManager) error {
	for i := 0; i < len(manager.Layers); i++ {
		if manager.Layers[i].Length() < s.MinMergeThreshold {
			continue
		}
		readers := make([]*sstable.SSTableReader, len(manager.Layers[i].SSTables))
		copy(readers, manager.Layers[i].SSTables)
		if i+1 == s.MaxHeight {
			if config.GetSettings().Debug {
				fmt.Printf("Compacting %d SSTables at layer %d\n", len(readers), i)
			}
			err := manager.MergeSSTables(readers, i, true)
			if err != nil {
				return err
			}
			return nil
		}
		if config.GetSettings().Debug {
			fmt.Printf("Compacting %d SSTables at layer %d\n", len(readers), i)
		}
		err := manager.MergeSSTables(readers, min(i+1, s.MaxHeight-1), false)
		if err != nil {
			return err
		}
	}
	return nil
}

func newLayer() *sstable.Layer {
	return &sstable.Layer{
		SSTables: make([]*sstable.SSTableReader, 0),
	}
}
