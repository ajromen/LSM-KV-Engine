package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ajromen/LSM-KV-Engine/internal/cli"
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
		c.Memtable.MemtableMaxSize = *flags.MemtableMaxSize
	}
	if flags.MemtableMaxSizeKb != nil {
		c.Memtable.MemtableSizeKB = *flags.MemtableMaxSizeKb
	}
	if flags.MemtableType != nil {
		c.Memtable.MemtableType = *flags.MemtableType
	}

	// Block cache
	if flags.BlockCacheMaxBlocks != nil {
		c.BlockManager.BlockCacheMaxBlocks = *flags.BlockCacheMaxBlocks
	}

	// BlockSize applies to WAL bcs they should match (??)
	if flags.BlockSize != nil {
		c.BlockManager.BlockSize = *flags.BlockSize
		c.WAL.BlockSize = *flags.BlockSize
	}

	// WAL overrides
	if flags.WALSegmentSize != nil {
		c.WAL.SegmentSize = *flags.WALSegmentSize
	}
	if flags.WALSyncInterval != nil {
		c.WAL.SyncInterval = *flags.WALSyncInterval
	}
	if flags.WALMaxSegments != nil {
		c.WAL.MaxSegments = *flags.WALMaxSegments
	}

	return nil
}

func (c *Config) validateFields() error {
	// Memtable validation
	if c.Memtable.MemtableMaxSize <= 0 &&
		c.Memtable.MemtableSizeKB <= 0 {
		return fmt.Errorf("memtable size must be positive")
	}

	if c.Memtable.MemtableType != "hashmap" &&
		c.Memtable.MemtableType != "skiplist" &&
		c.Memtable.MemtableType != "btree" {
		return fmt.Errorf("invalid memtable type")
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

	// WAL validation
	if c.WAL.BlockSize <= 0 {
		return fmt.Errorf("wal.block_size must be positive")
	}
	if c.WAL.SegmentSize <= 0 {
		return fmt.Errorf("wal.segment_size must be positive")
	}
	if c.WAL.SegmentSize < c.WAL.BlockSize {
		return fmt.Errorf("wal.segment_size (%d) must be >= wal.block_size (%d)", c.WAL.SegmentSize, c.WAL.BlockSize)
	}
	if c.WAL.SyncInterval < 0 {
		return fmt.Errorf("wal.sync_interval cannot be negative")
	}
	if c.WAL.MaxSegments < 0 {
		return fmt.Errorf("wal.max_segments cannot be negative")
	}

	// WAL BlockManager compatibility
	if c.WAL.BlockSize != c.BlockManager.BlockSize {
		return fmt.Errorf("wal.block_size (%d) must match blockmanager.block_size (%d)",
			c.WAL.BlockSize, c.BlockManager.BlockSize)
	}

	// TODO: druge funkcionalnosti
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
