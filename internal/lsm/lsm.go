package lsm

import (
	"time"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
	"github.com/ajromen/LSM-KV-Engine/internal/memtable"
	"github.com/ajromen/LSM-KV-Engine/internal/sstable"
)

type LSM struct {
	memtableeManager *memtable.MemtableManager
	sstableManager   *sstable.SSTableManager
	strategy         CompactionStrategy
	cfg              *config.Config
}

func NewLSM(cfg *config.Config, dataDir string) (*LSM, error) {
	lsm := LSM{cfg: cfg}
	lsm.sstableManager = sstable.NewSSTableManager(dataDir, cfg)
	switch cfg.LSMTree.CompactionAlgorithm {
	case enums.LeveledCompaction:
		lsm.strategy = LeveledCompaction{
			L1MaxBytes:          int64(cfg.Memtable.MemtableMaxSizeBytes) * int64(cfg.LSMTree.LevelSizeMultiplier),
			LevelSizeMultiplier: cfg.LSMTree.LevelSizeMultiplier,
			MaxHeight:           cfg.LSMTree.MaxHeight,
		}
	case enums.SizeTieredCompaction:
		lsm.strategy = SizeTiredCompaction{
			MinMergeThreshold: cfg.LSMTree.MinMergeThreshold,
			MaxHeight:         cfg.LSMTree.MaxHeight,
		}
	}

	factory := memtable.NewFactory(cfg.Memtable)
	memManager := memtable.NewMemtableManager(3, 0, factory, lsm.onFlush)
	lsm.memtableeManager = memManager
	return &lsm, nil
}

func (l *LSM) onFlush(entries []memtable.MemtableEntry) {
	err := l.sstableManager.FlushToSSTable(entries)
	if err != nil {
		panic(err)
	}

	// TODO set this in a goroutine
	err = l.strategy.Compact(l.sstableManager)
	if err != nil {
		panic(err)
	}
}

func (l *LSM) Put(key []byte, value []byte) error {
	ts := currentTimestamp()
	l.memtableeManager.Put(key, value, ts, false)
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

func (l *LSM) Delete(key []byte) error {
	ts := currentTimestamp()
	l.memtableeManager.Put(key, nil, ts, true)
	return nil
}

func (l *LSM) Finish() error {
	l.memtableeManager.Close()
	return nil
}

func currentTimestamp() uint64 {
	now := time.Now().UnixNano()
	return uint64(now)
}
