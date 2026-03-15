package config

import (
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
)

type Config struct {
	WAL               WALConfig               `json:"wal"`
	Memtable          MemtableConfig          `json:"memtable"`
	SSTable           SSTableConfig           `json:"sstable"`
	LSMTree           LSMTreeConfig           `json:"lsmtree"`
	BlockManager      BlockManagerConfig      `json:"blockmanager"`
	Snapshot          SnapshotConfig          `json:"snapshot"`
	Checkpoint        CheckpointConfig        `json:"checkpoint"`
	Backup            BackupConfig            `json:"backup"`
	ProbabilisticType ProbabilisticTypeConfig `json:"probabilistic_type"`
	SkipList          SkipListConfig          `json:"skiplist"`
	SavePath          string                  `json:"save_path"`
}

type WALConfig struct {
	WALSegmentSize int `json:"segment_size"`
}

type MemtableConfig struct {
	MemtableMaxEntries   int                `json:"memtable_max_size"`
	MemtableMaxSizeBytes uint64             `json:"memtable_max_size_bytes"`
	MemtableType         enums.MemTableType `json:"memtable_type"`
	Instances            int                `json:"instances"`
	SkipListConfig       SkipListConfig     `json:"skiplist_config"`
	BTreeConfig          BTreeConfig        `json:"btree_config"`
}

type SSTableConfig struct {
	Format       enums.SSTableFormat `json:"format"`
	DataSegment  DataSegmentConfig   `json:"data_segment"`
	IndexSegment IndexSegmentConfig  `json:"index_segment"`
}

type DataSegmentConfig struct {
	BlockSize           int                      `json:"block_size"`
	RestartInterval     int                      `json:"restart_interval"`
	Compression         enums.SSTableCompression `json:"compression"`
	MinBlockUtilization float64                  `json:"min_block_utilization"`
}

type IndexSegmentConfig struct {
	IndexBlockSize int `json:"index_block_size"`
	MaxCache       int `json:"max_cache_size"`
}

type LSMTreeConfig struct {
	MaxLevels           int                  `json:"lsmtree_max_levels"`
	CompactionAlgorithm enums.LSMCompression `json:"compaction_algorithm"`
}

type BlockManagerConfig struct {
	BlockSize           int `json:"block_size"`
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
	Enabled    bool     `json:"enabled"`
	Accuracy   float64  `json:"accuracy"`
	Confidence float64  `json:"confidence"`
	Seeds      [][]byte `json:"seeds"`
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

type BTreeConfig struct {
	MinimumDegree int `json:"minimum_degree"`
}
