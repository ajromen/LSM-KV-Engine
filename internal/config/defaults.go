package config

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/ajromen/LSM-KV-Engine/internal/enums"
)

// Za sva podešavanja koja nedostaju u konfiguracionom fajlu sistem treba da dodeli
// default vrednosti koje se navode u kodu

// Engine defaults
const (
	//WAL
	DefaultWalSegmentSize = 1 * 1024 * 1024

	//memtable
	MemtableType                = enums.HashMapMemTable
	DefaultMemtableMaxEntries   = 1000
	DefaultMemtableMaxSizeBytes = 1 << 20
	DefaultMemtableInstances    = 5
	DefaultSkipListMaxLevel     = 10
	DefaultBTreeMinimumDegree   = 8

	//SSTable
	DefaultSSTableBlockSize           = 160
	DefaultSSTableRestartInterval     = 3
	DefaultSSTableCompression         = enums.CompressionNone
	DefaultSSTableMinBlockUtilization = 0.8
	DefaultIndexBlockSize             = 160

	//CMS
	DefaultCMSAccuracy   = 0.01
	DefaultCMSConfidence = 0.99

	//Block Manager
	DefaultBlockSize           = 4 * 1024
	DefaultBlockCacheMaxBlocks = 2048

	//LSM
	DefaultLSMMaxLayers           = 4
	DefaultLSMCompactionAlgorithm = enums.SizeTieredCompaction
)

func NewDefaultConfig() *Config {
	return &Config{
		SavePath: getDefaultSavePath(),
		WAL: WALConfig{
			WALSegmentSize: DefaultWalSegmentSize,
		},
		Memtable: MemtableConfig{
			MemtableType:         MemtableType,
			MemtableMaxEntries:   DefaultMemtableMaxEntries,
			MemtableMaxSizeBytes: DefaultMemtableMaxSizeBytes,
			Instances:            DefaultMemtableInstances,
			SkipListConfig: SkipListConfig{
				MaxLevel: DefaultSkipListMaxLevel,
			},
			BTreeConfig: BTreeConfig{
				MinimumDegree: DefaultBTreeMinimumDegree,
			},
		},
		SSTable: SSTableConfig{
			Format: enums.FormatSingleFile,
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

func getDefaultSavePath() string {
	var path string
	switch runtime.GOOS {
	case "windows":
		// C:\ProgramData\lsm-kv-engine
		base := os.Getenv("ProgramData")
		if base == "" {
			base = `C:\ProgramData`
		}
		path = filepath.Join(base, "lsm-kv-engine")
	case "darwin":
		// ~/Library/Application Support/lsm-kv-engine
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, "Library", "Application Support", "lsm-kv-engine")
	default:
		// ~/.local/share/lsm-kv-engine
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, "lsm-kv-engine")
		}
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".local", "share", "lsm-kv-engine")
	}
	err := os.MkdirAll(path, 0755)
	if err != nil {
		panic(err)
	}
	return path
}
