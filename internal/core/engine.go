package core

import (
	"os"
	"time"

	"github.com/ajromen/LSM-KV-Engine/internal/cli"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/memtable"
	"github.com/ajromen/LSM-KV-Engine/internal/sstable"
)

type Engine struct {
	config          *config.Config
	memtableFactory *memtable.MemtableFactory
	sstableFactory  *sstable.SSTableFactory
}

func currentTimestamp() uint64 {
	now := time.Now().UnixNano() // nanosekunde od 1.1.1970
	return uint64(now)
}

func NewEngine(flags *cli.FLags) (*Engine, error) {
	cfg, err := config.LoadConfig(flags)
	if err != nil {
		return nil, err
	}
	dataDir := "./data"
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}
	sstableFactory := sstable.NewSSTableFactory(dataDir, cfg)
	factory := memtable.NewFactory("skiplist", 10, 1<<20, cfg.Memtable)
	flushHandler := func(entries []memtable.MemtableEntry) {
		if err := sstableFactory.FlushToSSTable(entries); err != nil {
			return
		}
	}
	memFactory := memtable.NewMemtableFactory(3, 0, factory, flushHandler)
	engine := &Engine{
		config:          cfg,
		memtableFactory: memFactory,
		sstableFactory:  sstableFactory,
	}
	return engine, nil
}

func (engine *Engine) Put(key []byte, value []byte) error {
	ts := currentTimestamp()
	engine.memtableFactory.Put(key, value, ts, false)
	return nil
}

func (engine *Engine) Get(key []byte) ([]byte, bool, error) {
	entry, found := engine.memtableFactory.Get(key)
	if found {
		return entry, true, nil
	}
	entry, found, err := engine.sstableFactory.Get(key)
	if err != nil {
		return nil, false, err
	}
	if found {
		return entry, true, nil
	}
	return nil, false, nil
}

func (engine *Engine) Delete(key []byte) error {
	ts := currentTimestamp()
	engine.memtableFactory.Put(key, nil, ts, true)
	return nil
}

func (engine *Engine) Close() {
}
