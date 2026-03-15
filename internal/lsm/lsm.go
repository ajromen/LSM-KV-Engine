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
	case enums.SizeTieredCompaction:
		lsm.strategy = LeveledCompaction{} //TODO LevelSizeMultiplier
	case enums.LeveledCompaction:
		lsm.strategy = SizeTiredCompaction{} //TODO MinMergeTreshold
	}

	factory := memtable.NewFactory(cfg.Memtable)
	memManager := memtable.NewMemtableManager(3, 0, factory, lsm.onFlush)
	lsm.memtableeManager = memManager
	return &lsm, nil
}

func (l *LSM) onFlush(entries []memtable.MemtableEntry) {
	l.sstableManager.FlushToSSTable(entries)

	// set this in a goroutine
	if l.strategy.ShouldCompact(l.sstableManager.Layers) {
		l.strategy.Compact(l.sstableManager.Layers)
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
	return nil
}

func currentTimestamp() uint64 {
	now := time.Now().UnixNano() // nanosekunde od 1.1.1970
	return uint64(now)
}
