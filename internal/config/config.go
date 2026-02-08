package config

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
	DataSegment DataSegmentConfig
}

type DataSegmentConfig struct {
	BlockSize           int     `json:"block_size"`
	RestartInterval     int     `json:"restart_interval"`
	Compression         string  `json:"compression"`
	CompressionLevel    int     `json:"compression_level"` // za zstd
	MinBlockUtilization float64 `json:"min_block_utilization"`
}

type LSMTreeConfig struct {
	// Ovde ce jos trebati sa Compaction raditi
	LSMTreeMaxLevels int `json:"lsmtree_max_levels"`
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
