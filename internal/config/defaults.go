package config

// Za sva podešavanja koja nedostaju u konfiguracionom fajlu sistem treba da dodeli
// default vrednosti koje se navode u kodu
const (
	CompressionNone   byte = 0
	CompressionSnappy byte = 1
	CompressionZSTD   byte = 2
)

const (
	FormatSingleFile byte = 0
	FormatMultiFile  byte = 1
)

// Engine defaults
const (
	//WAL
	DefaultWalSegmentSize = 1 * 1024 * 1024

	//memtable
	MemtableType              = "hashmap"
	DefaultMemtableMaxEntries = 1000
	DefaultMemtableInstances  = 5
	DefaultSkipListMaxLevel   = 10
	DefaultBTreeMinimumDegree = 8

	//SSTable
	DefaultSSTableBlockSize           = 160
	DefaultSSTableRestartInterval     = 3
	DefaultSSTableCompression         = CompressionNone
	DefaultSSTableMinBlockUtilization = 0.8
	DefaultIndexBlockSize             = 20

	//CMS
	DefaultCMSAccuracy   = 0.01
	DefaultCMSConfidence = 0.99

	//Block Manager
	DefaultBlockSize           = 4 * 1024
	DefaultBlockCacheMaxBlocks = 2048

	//LSM
	DefaultLSMMaxLayers           = 4
	DefaultLSMCompactionAlgorithm = "size-tiered"
)

func NewDefaultConfig() *Config {
	return &Config{
		WAL: WALConfig{
			WALSegmentSize: DefaultWalSegmentSize,
		},
		Memtable: MemtableConfig{
			MemtableType:    MemtableType,
			MemtableMaxSize: DefaultMemtableMaxEntries,
			Instances:       DefaultMemtableInstances,
			SkipListConfig: SkipListConfig{
				MaxLevel: DefaultSkipListMaxLevel,
			},
			BTreeConfig: BTreeConfig{
				MinimumDegree: DefaultBTreeMinimumDegree,
			},
		},
		SSTable: SSTableConfig{
			Format: FormatSingleFile,
			DataSegment: DataSegmentConfig{
				BlockSize:           DefaultSSTableBlockSize,
				RestartInterval:     DefaultSSTableRestartInterval,
				Compression:         DefaultSSTableCompression,
				MinBlockUtilization: DefaultSSTableMinBlockUtilization,
			},
			IndexSegment: IndexSegmentConfig{
				IndexBlockSize: DefaultIndexBlockSize,
			},
		},
		LSMTree: LSMTreeConfig{
			MaxLevels:           DefaultLSMMaxLayers,
			CompactionAlgorithm: DefaultLSMCompactionAlgorithm,
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
