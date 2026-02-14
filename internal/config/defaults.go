package config

// Engine defaults
const (
	// WAL
	DefaultWalSegmentSize = 1 * 1024 * 1024 // bytes

	// Memtable
	MemtableType              = "hashmap"
	DefaultMemtableMaxEntries = 1000

	// SSTable
	DefaultSSTableBlockSize = 16

	// SkipList
	DefaultSkipListMaxLevel = 16

	// CMS
	DefaultCMSAccuracy   = 0.01
	DefaultCMSConfidence = 0.99

	// Block Manager
	DefaultBlockSize           = 4 * 1024 // bytes
	DefaultBlockCacheMaxBlocks = 2048
)

func NewDefaultConfig() *Config {
	return &Config{
		WAL: WALConfig{
			SegmentSize:  DefaultWalSegmentSize,
			BlockSize:    DefaultBlockSize, // MUST match BlockManager.BlockSize
			SyncInterval: 0,                // ms; 0 => only on Flush/Commit
			MaxSegments:  0,                // 0 => unlimited
		},
		Memtable: MemtableConfig{
			MemtableType:    MemtableType,
			MemtableMaxSize: DefaultMemtableMaxEntries,
		},
		SSTable: SSTableConfig{
			SSTableDataBlockSize: DefaultSSTableBlockSize,
		},
		SkipList: SkipListConfig{
			MaxLevel: DefaultSkipListMaxLevel,
		},
		ProbabilisticType: ProbabilisticTypeConfig{
			CountMinSketch: CountMinSketchConfig{
				Enabled:    true,
				Accuracy:   DefaultCMSAccuracy,
				Confidence: DefaultCMSConfidence,
				Seeds: [][]byte{
					{1, 2, 3, 4},
					{5, 6, 7, 8},
					{9, 10, 11, 12},
				},
			},
		},
		BlockManager: BlockManagerConfig{
			BlockSize:           DefaultBlockSize,
			BlockCacheMaxBlocks: DefaultBlockCacheMaxBlocks,
		},
	}
}
