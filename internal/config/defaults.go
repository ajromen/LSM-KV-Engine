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

	//SSTable
	DefaultSSTableBlockSize           = 16
	DefaultSSTableRestartInterval     = 4
	DefaultSSTableCompression         = CompressionNone
	DefaultSSTableMinBlockUtilization = 0.8

	//SkipList
	DefaultSkipListMaxLevel = 16

	//CMS
	DefaultCMSAccuracy   = 0.01
	DefaultCMSConfidence = 0.99

	//Block Manager
	DefaultBlockSize           = 4 * 1024
	DefaultBlockCacheMaxBlocks = 2048
)

func NewDefaultConfig() *Config {
	return &Config{
		WAL: WALConfig{
			WALSegmentSize: DefaultWalSegmentSize,
		},
		Memtable: MemtableConfig{
			MemtableType:    MemtableType,
			MemtableMaxSize: DefaultMemtableMaxEntries,
		},
		SSTable: SSTableConfig{
			Format: FormatSingleFile,
			DataSegment: DataSegmentConfig{
				BlockSize:           DefaultSSTableBlockSize,
				RestartInterval:     DefaultSSTableRestartInterval,
				Compression:         DefaultSSTableCompression,
				MinBlockUtilization: DefaultSSTableMinBlockUtilization,
			},
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

//func applyDefaults(cfg *Config) {
//	if cfg.WAL.WALSegmentSize == 0 {
//		cfg.WAL.WALSegmentSize = 1024 * 1024
//	}
//	// Ovde kaze "Maksimalnu velicinu specificira korisnik tako sto navodi broj elemenata ILI zauzece memorije u KB" - na
//	// nama je da vidimo hocemo li birati jedno od ta dva ili proveravati koji je popunjen i dati koristiti
//	if cfg.Memtable.MemtableMaxSize == 0 && cfg.Memtable.MemtableSizeKB == 0 {
//		cfg.Memtable.MemtableMaxSize = 1000
//	}
//	if cfg.Memtable.MemtableType == "" {
//		cfg.Memtable.MemtableType = "hashmap"
//	}
//	if cfg.SSTable.SSTableDataBlockSize == 0 {
//		cfg.SSTable.SSTableDataBlockSize = 16 // Ovo je u KB (Tako je na LevelDB pa kontam da je ok)
//	}
//	if cfg.SkipList.MaxLevel == 0 {
//		cfg.SkipList.MaxLevel = 16
//	}
//}
