package lsm

import (
	"fmt"

	"github.com/ajromen/LSM-KV-Engine/internal/cache"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
	"github.com/ajromen/LSM-KV-Engine/internal/iterator"
	"github.com/ajromen/LSM-KV-Engine/internal/memtable"
	"github.com/ajromen/LSM-KV-Engine/internal/sstable"
	"github.com/ajromen/LSM-KV-Engine/internal/ttl"
)

type LSM struct {
	memtableeManager *memtable.MemtableManager
	sstableManager   *sstable.SSTableManager
	strategy         CompactionStrategy
	readCache        *cache.LRU[string, []byte]
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
	lsm.readCache = cache.NewLRU[string, []byte](stt.LSMTree.ReadCacheSize)
	return &lsm, nil
}

func (l *LSM) onFlush(entries []memtable.MemtableEntry, rangeDelEntries []memtable.MemtableEntry) {
	err := l.sstableManager.FlushToSSTable(entries, rangeDelEntries)
	if err != nil {
		panic(fmt.Errorf("error flushing memtable entries: %v", err))
	}

	// TODO set this in a goroutine
	err = l.strategy.Compact(l.sstableManager)
	if err != nil {
		panic(err)
	}
}

func (l *LSM) Put(key []byte, value []byte, seqId uint64, opType enums.OpType) {
	l.memtableeManager.Put(key, value, seqId, opType)
	if opType == enums.OpTypeDel {
		l.readCache.Put(string(key), nil)
	} else if opType == enums.OpTypeRangeDel {
		l.readCache.Put(string(key), nil)
	} else {
		l.readCache.Put(string(key), value)
	}
}

func (l *LSM) PutWithTTL(key []byte, value []byte, seqId uint64, opType enums.OpType, ttl int64) {
	l.memtableeManager.PutWithTTL(key, value, seqId, opType, ttl)
}

func (l *LSM) Get(key []byte) ([]byte, bool, error) {
	// 1. check memtable
	entry, found := l.memtableeManager.Get(key)
	if found {
		if entry == nil {
			l.readCache.Put(string(key), nil)
			return nil, false, nil
		}
		l.readCache.Put(string(key), entry.Value)
		return entry.Value, true, nil
	}

	if config.GetSettings().Debug {
		fmt.Printf("Key not found in memtable checking cache\n")
	}

	// 2. check cache
	if val, ok := l.readCache.Get(string(key)); ok {
		if val == nil {
			return nil, false, nil
		}
		return val, true, nil
	}

	if config.GetSettings().Debug {
		fmt.Printf("Key not found in cache checking sstable\n")
	}

	// 3. check SSTable
	record, found, err := l.sstableManager.Get(key)
	if err != nil {
		return nil, false, err
	}
	if found {
		// The key exists in SSTable but a range tombstone in the memtable
		// may have been written after it. Check with the record's own seqId.
		if l.memtableeManager.IsCoveredByRangeDel(key, record.SeqId) {
			return nil, false, nil
		}
		l.readCache.Put(string(key), record.Value)
		return record.Value, true, nil
	}

	return nil, false, nil
}

func (l *LSM) GetTTL(key []byte) (int64, bool, error) {
	entry, found := l.memtableeManager.Get(key)
	if found {
		if entry == nil {
			return 0, false, nil // tombstone
		}
		return entry.ExpiresAt, true, nil
	}
	record, found, err := l.sstableManager.Get(key)
	if err != nil {
		return 0, false, err
	}
	if found {
		return record.ExpiresAt, true, nil
	}
	return 0, false, nil
}

// flushes memtable
func (l *LSM) Finish() error {
	l.memtableeManager.Close()
	return nil
}

func (l *LSM) ClearAll() error {
	l.readCache.Clear()
	l.memtableeManager.ResetAll()
	return l.sstableManager.ClearAll()
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

func (l *LSM) GetAllTTLFomSST() (*ttl.ExpiryHeap, map[string]int64, error) {
	return l.sstableManager.GetAllTTL()
}

func (l *LSM) GetManifest() sstable.Manifest {
	return *l.sstableManager.Manifest
}
