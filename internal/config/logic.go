package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ajromen/LSM-KV-Engine/internal/cli"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
)

func LoadConfig(flags *cli.FLags) error {
	cfg := NewDefaultConfig()
	if flags.ConfigPath != nil {
		err := cfg.loadFromFile(*flags.ConfigPath)
		if err != nil {
			return err
		}
	}
	err := cfg.applyFlags(flags)
	if err != nil {
		return err
	}

	err = cfg.validateFields()
	if err != nil {
		return err
	}
	createSettings(cfg)
	return nil
}

func (c *Config) applyFlags(flags *cli.FLags) error {
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
	if flags.TTLInMemoryTTL != nil {
		c.TTL.InMemoryTTL = *flags.TTLInMemoryTTL
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

	if c.BlockManager.BlockSize%(4*1024) != 0 {
		return fmt.Errorf("invalid block size, must be multiple of 4kb")
	}

	if c.BlockManager.BlockCacheMaxBlocks < 1 {
		return fmt.Errorf("invalid block cacheMaxBlocks must be positive")
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

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}
