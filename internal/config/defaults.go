package config

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/ajromen/LSM-KV-Engine/internal/enums"
)

// Engine defaults
const (
	defaultDebug = false
	//WAL
	defaultWalSegmentSize = 1 * 1024 * 1024

	//memtable
	memtableType                = enums.HashMapMemTable
	defaultMemtableMaxEntries   = 1000
	defaultMemtableMaxSizeBytes = 1
	defaultMemtableInstances    = 5
	defaultSkipListMaxLevel     = 10
	defaultBTreeMinimumDegree   = 8

	//SSTable
	defaultSSTableBlockSize           = 160
	defaultSSTableRestartInterval     = 3
	defaultSSTableCompression         = enums.CompressionNone
	defaultSSTableMinBlockUtilization = 0.8

	//CMS
	defaultCMSAccuracy   = 0.01
	defaultCMSConfidence = 0.99

	//Block Manager
	defaultBlockSize           = 4 * 1024
	defaultBlockCacheMaxBlocks = 2048

	//LSM
	defaultLSMCompactionAlgorithm = enums.SizeTieredCompaction
	defaultLSMMinMergeThreshold   = 4
	defaultLSMLevelSizeMultiplier = 10
	defaultMaxLSMHeight           = 5

	defaultReadCacheSize = 1000

	//TTL
	defaultTTLInMemoryTTL = true
	defaultTTLRefreshRate = 1000 // 1s

	// Token Bucket
	defaultTokenBucketMaxTokens       int64 = 0    // 0
	defaultTokenBucketResetIntervalMs int64 = 1000 // 1s
)

func NewDefaultConfig() *Config {
	return &Config{
		SavePath: getDefaultSavePath(),
		Debug:    defaultDebug,
		WAL: WALConfig{
			WALSegmentSize: defaultWalSegmentSize,
		},
		Memtable: MemtableConfig{
			MemtableType:         memtableType,
			MemtableMaxEntries:   defaultMemtableMaxEntries,
			MemtableMaxSizeBytes: defaultMemtableMaxSizeBytes,
			Instances:            defaultMemtableInstances,
			SkipListConfig: SkipListConfig{
				MaxLevel: defaultSkipListMaxLevel,
			},
			BTreeConfig: BTreeConfig{
				MinimumDegree: defaultBTreeMinimumDegree,
			},
		},
		SSTable: SSTableConfig{
			Format: enums.FormatSingleFile,
			DataSegment: DataSegmentConfig{
				BlockSize:           defaultSSTableBlockSize,
				RestartInterval:     defaultSSTableRestartInterval,
				Compression:         defaultSSTableCompression,
				MinBlockUtilization: defaultSSTableMinBlockUtilization,
			},
			IndexSegment: IndexSegmentConfig{},
		},
		LSMTree: LSMTreeConfig{
			MaxHeight:           defaultMaxLSMHeight,
			MinMergeThreshold:   defaultLSMMinMergeThreshold,
			CompactionAlgorithm: defaultLSMCompactionAlgorithm,
			LevelSizeMultiplier: defaultLSMLevelSizeMultiplier,
			ReadCacheSize:       defaultReadCacheSize,
		},
		SkipList: SkipListConfig{
			MaxLevel: defaultSkipListMaxLevel,
		},
		ProbabilisticType: ProbabilisticTypeConfig{
			CountMinSketch: CountMinSketchConfig{
				Accuracy:   defaultCMSAccuracy,
				Confidence: defaultCMSConfidence,
				Seeds: [][]byte{
					{1, 2, 3, 4},
					{5, 6, 7, 8},
					{9, 10, 11, 12},
				},
			},
		},
		BlockManager: BlockManagerConfig{
			BlockSize:           defaultBlockSize,
			BlockCacheMaxBlocks: defaultBlockCacheMaxBlocks,
		},
		TTL: TTLConfig{
			InMemoryTTL: defaultTTLInMemoryTTL,
			RefreshRate: defaultTTLRefreshRate,
		},
		TokenBucket: TokenBucketConfig{
			MaxTokens:       defaultTokenBucketMaxTokens,
			ResetIntervalMs: defaultTokenBucketResetIntervalMs,
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
