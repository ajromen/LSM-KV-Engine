package core

import (
	"fmt"
	"os"
	"time"

	"github.com/ajromen/LSM-KV-Engine/internal/cli"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/memtable"
	"github.com/ajromen/LSM-KV-Engine/internal/sstable"
)

type Engine struct {
	config          *config.Config
	memtableManager *memtable.MemtableManager
	sstableManager  *sstable.SSTableManager
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
	sstableManager := sstable.NewSSTableManager(dataDir, cfg)
	factory := memtable.NewFactory(cfg.Memtable)
	flushHandler := func(entries []memtable.MemtableEntry) {
		if err := sstableManager.FlushToSSTable(entries); err != nil {
			return
		}
	}
	memManager := memtable.NewMemtableManager(3, 0, factory, flushHandler)
	engine := &Engine{
		config:          cfg,
		memtableManager: memManager,
		sstableManager:  sstableManager,
	}
	return engine, nil
}

func (engine *Engine) Put(key []byte, value []byte) error {
	ts := currentTimestamp()
	engine.memtableManager.Put(key, value, ts, false)
	return nil
}

func (engine *Engine) Get(key []byte) ([]byte, bool, error) {
	entry, found := engine.memtableManager.Get(key)
	if found {
		return entry, true, nil
	}
	entry, found, err := engine.sstableManager.Get(key)
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
	engine.memtableManager.Put(key, nil, ts, true)
	return nil
}

func (engine *Engine) Close() {
}

func (engine *Engine) DataRaw() {
	dataF, err := os.Open("data/000001.sst.data")
	if err != nil {
		fmt.Println("open error:", err)
		return
	}
	defer dataF.Close()

	stats, err := dataF.Stat()
	if err != nil {
		fmt.Println("stat error:", err)
		return
	}

	buf := make([]byte, int(stats.Size()))

	n, err := dataF.ReadAt(buf, 0)
	if err != nil {
		fmt.Println("read error:", err)
		return
	}

	buf = buf[:n]

	fmt.Println("Data bytes:", buf)
}
