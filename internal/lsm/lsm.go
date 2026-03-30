package lsm

import (
	"fmt"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
	"github.com/ajromen/LSM-KV-Engine/internal/iterator"
	"github.com/ajromen/LSM-KV-Engine/internal/memtable"
	"github.com/ajromen/LSM-KV-Engine/internal/sstable"
)

type LSM struct {
	memtableeManager *memtable.MemtableManager
	sstableManager   *sstable.SSTableManager
	strategy         CompactionStrategy
}

func NewLSM(dataDir string) (*LSM, error) {
	lsm := LSM{}
	lsm.sstableManager = sstable.NewSSTableManager(dataDir)
	stt := config.GetSettings()
	switch stt.LSMTree.CompactionAlgorithm {
	case enums.LeveledCompaction:
		lsm.strategy = LeveledCompaction{
			L1MaxBytes:          int64(stt.Memtable.MemtableMaxSizeBytes) * int64(stt.LSMTree.LevelSizeMultiplier),
			LevelSizeMultiplier: stt.LSMTree.LevelSizeMultiplier,
			MaxHeight:           stt.LSMTree.MaxHeight,
		}
	case enums.SizeTieredCompaction:
		lsm.strategy = SizeTiredCompaction{
			MinMergeThreshold: stt.LSMTree.MinMergeThreshold,
			MaxHeight:         stt.LSMTree.MaxHeight,
		}
	}

	factory := memtable.NewFactory(stt.Memtable)
	memManager := memtable.NewMemtableManager(3, 0, factory, lsm.onFlush)
	lsm.memtableeManager = memManager
	return &lsm, nil
}

func (l *LSM) onFlush(entries []memtable.MemtableEntry) {
	err := l.sstableManager.FlushToSSTable(entries)
	if err != nil {
		panic(fmt.Errorf("error flushing memtable entries: %v", err))
	}

	// TODO set this in a goroutine
	err = l.strategy.Compact(l.sstableManager)
	if err != nil {
		panic(err)
	}
}

func (l *LSM) Put(key []byte, value []byte, seqId uint64) error {
	l.memtableeManager.Put(key, value, seqId, false)
	return nil
}

func (l *LSM) Get(key []byte) ([]byte, bool, error) {
	entry, found := l.memtableeManager.Get(key)
	if found {
		if entry == nil {
			return nil, false, nil // tombstone
		}
		return entry, true, nil
	}
	entry, found, err := l.sstableManager.Get(key)
	if err != nil {
		return nil, false, err
	}
	if found {
		return entry, true, nil
	}
	return nil, false, nil
}

func (l *LSM) Delete(key []byte, seqId uint64) error {
	l.memtableeManager.Put(key, nil, seqId, true)
	return nil
}

func (l *LSM) Finish() error {
	l.memtableeManager.Close()
	return nil
}

func (l *LSM) GetMaxSeqId() uint64 {
	return l.sstableManager.Manifest.MaxSeqId
}

// NewDBIterator creates a DBIterator that merges memtable and sstable data
// start/end are used to filter which SSTables to include
func (l *LSM) NewDBIterator(start, end []byte) (*iterator.DBIterator, error) {
	memIt := l.memtableeManager.EntryIterator()
	sstIt, err := l.sstableManager.EntryIterator(start, end)
	if err != nil {
		return nil, err
	}
	return iterator.NewDBIterator(memIt, sstIt), nil
}

// NewRangeIterator creates a RangeIterator over [lower, upper]
func (l *LSM) NewRangeIterator(lower, upper []byte) (*iterator.RangeIterator, error) {
	dbIt, err := l.NewDBIterator(lower, upper)
	if err != nil {
		return nil, err
	}
	return iterator.NewRangeIterator(dbIt, lower, upper), nil
}

// NewPrefixIterator creates a PrefixIterator for the given prefix
func (l *LSM) NewPrefixIterator(prefix []byte) (*iterator.PrefixIterator, error) {
	upper := append(append([]byte(nil), prefix...), 0xFF)
	dbIt, err := l.NewDBIterator(prefix, upper)
	if err != nil {
		return nil, err
	}
	return iterator.NewPrefixIterator(dbIt, prefix), nil
}
