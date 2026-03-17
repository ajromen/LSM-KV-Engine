package compaction

import (
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/memtable"
	"github.com/ajromen/LSM-KV-Engine/internal/sstable"
)

type Layer struct {
	tables []sstable.SSTableReader
}

type LSMManager struct {
	memtableMaganer *memtable.MemtableManager
	sstableManager  *sstable.SSTableManager
	strategy        CompactionStrategy
	cfg             *config.Config
}

func NewLSMManager(cfg *config.Config, datadir string) *LSMManager {
	//memManager := memtable.NewMemtableManager(datadir, cfg)
	return &LSMManager{}
}
