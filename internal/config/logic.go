package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ajromen/LSM-KV-Engine/internal/cli"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
)

func LoadConfig(flags *cli.FLags) (*Config, error) {
	cfg := NewDefaultConfig()

	// load JSON if provided
	if flags != nil && flags.ConfigPath != nil {
		err := cfg.loadFromFile(*flags.ConfigPath)
		if err != nil {
			return nil, err
		}
	}

	// apply CLI overrides
	if flags != nil {
		if err := cfg.applyFlags(flags); err != nil {
			return nil, err
		}
	}

	// validate final config
	if err := cfg.validateFields(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) applyFlags(flags *cli.FLags) error {
	// Memtable
	if flags.MemtableMaxSize != nil {
		c.Memtable.MemtableMaxEntries = *flags.MemtableMaxSize
	}
	if flags.MemtableMaxSizeKb != nil {
		c.Memtable.MemtableMaxSizeBytes = *flags.MemtableMaxSizeKb
	}
	if flags.MemtableType != nil {
		memType := strings.TrimSpace(*flags.MemtableType)
		memType = strings.ToLower(memType)
		var memtableType enums.MemTableType

		switch memType {
		case "hashmap":
			memtableType = enums.HashMapMemTable
		case "skiplist":
			memtableType = enums.SkiplistMemTable
		case "btree":
			memtableType = enums.BTreeMemTable
		case "rbtree":
			memtableType = enums.RBTreeMemTable
		case "avltree":
			memtableType = enums.AVLTreeMemTable
		default:
			return fmt.Errorf("invalid memtable type: %s", memType)
		}
		c.Memtable.MemtableType = memtableType
	}
	if flags.Instances != nil {
		c.Memtable.Instances = *flags.Instances
	}
	if flags.SSTableFormat != nil {
		format := strings.TrimSpace(*flags.SSTableFormat)
		format = strings.ToLower(format)
		switch format {
		case "single-file":
			c.SSTable.Format = enums.FormatSingleFile
		case "multi-file":
			c.SSTable.Format = enums.FormatMultiFile
		default:
			return fmt.Errorf("invalid sstable format: %s", format)
		}
	}

	// Block cache
	if flags.BlockCacheMaxBlocks != nil {
		c.BlockManager.BlockCacheMaxBlocks = *flags.BlockCacheMaxBlocks
	}
	if flags.LSMCompactionAlgorithm != nil {
		algo := strings.TrimSpace(*flags.LSMCompactionAlgorithm)
		algo = strings.ToLower(algo)
		var algorithm enums.LSMCompaction
		switch algo {
		case "size-tiered":
			algorithm = enums.SizeTieredCompaction
		case "leveled":
			algorithm = enums.LeveledCompaction
		default:
			return fmt.Errorf("unknown LSM compaction algorithm: %s", algo)
		}
		c.LSMTree.CompactionAlgorithm = algorithm
	}
	return nil
}

func (c *Config) validateFields() error {

	if c.Memtable.MemtableMaxEntries <= 0 {
		return fmt.Errorf("memtable size entries must be positive")
	}
	if c.Memtable.MemtableMaxSizeBytes <= 0 {
		return fmt.Errorf("memtable size bytes must be positive")
	}
	if c.Memtable.Instances <= 0 {
		return fmt.Errorf("instances must be positive")
	}

	if c.SSTable.Format != enums.FormatSingleFile && c.SSTable.Format != enums.FormatMultiFile {
		return fmt.Errorf("invalid sstable format")
	}

	// BlockManager validation
	if c.BlockManager.BlockSize <= 0 {
		return fmt.Errorf("blockmanager.block_size must be positive")
	}
	// multiple of 4KB
	if c.BlockManager.BlockSize%(4*1024) != 0 {
		return fmt.Errorf("invalid block size, must be multiple of 4kb")
	}

	if c.BlockManager.BlockCacheMaxBlocks < 1 {
		return fmt.Errorf("invalid blockcache_max_blocks must be positive")
	}

	if !fileExists(c.SavePath) {
		return fmt.Errorf("save path does not exist")
	}

	// TODO continue validation
	return nil
}

func (c *Config) loadFromFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot open config file: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(c); err != nil {
		return fmt.Errorf("invalid config format: %w", err)
	}
	return nil
}

//func (c *SSTableConfig) SegmentPaths(basePath string) map[enums.SegmentType]string {
//	paths := make(map[enums.SegmentType]string)
//	if c.Format == enums.FormatSingleFile {
//		for _, segType := range []enums.SegmentType{
//			enums.SegmentData, enums.SegmentFilter, enums.SegmentIndex,
//			enums.SegmentSummary, enums.SegmentMetadata, enums.SegmentFooter,
//		} {
//			paths[segType] = basePath
//		}
//	} else {
//		// Each segment has its own file
//		paths[enums.SegmentData] = basePath + string(sstable.DataSegmentExtension)
//		paths[enums.SegmentFilter] = basePath + string(sstable.FilterSegmentExtension)
//		paths[enums.SegmentIndex] = basePath + string(sstable.IndexSegmentExtension)
//		paths[enums.SegmentSummary] = basePath + string(sstable.SummarySegmentExtension)
//		paths[enums.SegmentMetadata] = basePath + string(sstable.MetadataSegmentExtension)
//		paths[enums.SegmentFooter] = basePath + string(sstable.FooterSegmentExtension)
//	}
//	return paths
//}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}
