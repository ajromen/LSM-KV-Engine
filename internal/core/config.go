package core

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	WAL               WALConfig               `json:"wal"`
	Memtable          MemtableConfig          `json:"memtable"`
	SSTable           SSTableConfig           `json:"sstable"`
	LSMTree           LSMTreeConfig           `json:"lsmtree"`
	Cache             CacheConfig             `json:"cache"`
	BlockManager      BlockManagerConfig      `json:"blockmanager"`
	BlockCache        BlockCacheConfig        `json:"blockcache"`
	Snapshot          SnapshotConfig          `json:"snapshot"`
	Checkpoint        CheckpointConfig        `json:"checkpoint"`
	Backup            BackupConfig            `json:"backup"`
	ProbabilisticType ProbabilisticTypeConfig `json:"probabilistic_type"`
	SkipList          SkipListConfig          `json:"skiplist"`
}

type WALConfig struct {
	WALSegmentSize int `json:"segment_size"`
}

type MemtableConfig struct {
	MemtableMaxSize int    `json:"memtable_max_size"`
	MemtableType    string `json:"memtable_type"`
	MemtableSizeKB  int    `json:"memtable_size_kb"`
	Instances       int    `json:"instances"`
}

type SSTableConfig struct {
	// Ovo treba imati Index/Filter/Summary/Metadata... samo ne znam jos nista o tome
	SSTableDataBlockSize int `json:"data_block_size"`
}

type LSMTreeConfig struct {
	// Ovde ce jos trebati sa Compaction raditi
	LSMTreeMaxLevels int `json:"lsmtree_max_levels"`
}

type CacheConfig struct {
	CacheMaxSize int `json:"cache_max_size"`
}

type BlockManagerConfig struct {
	BlockSize int `json:"block_size"`
}

type BlockCacheConfig struct {
	BlockCacheMaxBlocks int `json:"blockcache_max_blocks"`
}

type SnapshotConfig struct {
	SnapshotEnabled bool `json:"snapshot_enabled"`
}

type CheckpointConfig struct {
	CheckpointEnabled bool `json:"checkpoint_enabled"`
}

type BackupConfig struct {
	BackupEnabled     bool `json:"backup_enabled"`
	BackupIncremental bool `json:"backup_incremental"`
}

// Treba odraditi i ovaj TokenBucket

type ProbabilisticTypeConfig struct {
	BloomFilter    BloomFilterConfig    `json:"bloomfilter"`
	CountMinSketch CountMinSketchConfig `json:"countminsketch"`
	HyperLogLog    HyperLogLogConfig    `json:"hyperloglog"`
	SimHash        SimHashConfig        `json:"simhash"`
}

type BloomFilterConfig struct {
	Enabled           bool    `json:"enabled"`
	FalsePositiveRate float32 `json:"false_positive_rate"`
}

type CountMinSketchConfig struct {
	Enabled bool `json:"enabled"`
}

type HyperLogLogConfig struct {
	Enabled bool `json:"enabled"`
}

type SimHashConfig struct {
	Enabled bool `json:"enabled"`
}

type TTLConfig struct {
	Enabled  bool `json:"enabled"`
	Duration int  `json:"duration"`
}

type SkipListConfig struct {
	MaxLevel int `json:"max_level"`
}

func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open config file: %w", err)
	}
	defer file.Close()
	var cfg Config
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("invalid config format: %w", err)
	}
	applyDefaults(&cfg)

	// if err := validateConfig(&cfg); err != nil { return nil,err } - napraviti validaciju config,json fajla

	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.WAL.WALSegmentSize == 0 {
		cfg.WAL.WALSegmentSize = 1024 * 1024
	}
	// Ovde kaze "Maksimalnu velicinu specificira korisnik tako sto navodi broj elemenata ILI zauzece memorije u KB" - na
	// nama je da vidimo hocemo li birati jedno od ta dva ili proveravati koji je popunjen i dati koristiti
	if cfg.Memtable.MemtableMaxSize == 0 && cfg.Memtable.MemtableSizeKB == 0 {
		cfg.Memtable.MemtableMaxSize = 1000
	}
	if cfg.Memtable.MemtableType == "" {
		cfg.Memtable.MemtableType = "hashmap"
	}
	if cfg.SSTable.SSTableDataBlockSize == 0 {
		cfg.SSTable.SSTableDataBlockSize = 16 // Ovo je u KB (Tako je na LevelDB pa kontam da je ok)
	}
	if cfg.SkipList.MaxLevel == 0 {
		cfg.SkipList.MaxLevel = 16
	}
}
