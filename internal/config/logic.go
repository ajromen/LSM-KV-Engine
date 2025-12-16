package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ajromen/LSM-KV-Engine/internal/cli"
)

func LoadConfig(flags *cli.FLags) (*Config, error) {
	cfg := NewDefaultConfig()
	if flags.ConfigPath != nil {
		err := cfg.loadFromFile(*flags.ConfigPath)
		if err != nil {
			return nil, err
		}
	}
	err := cfg.applyFlags(flags)
	if err != nil {
		return nil, err
	}

	err = cfg.validateFields()
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) applyFlags(flags *cli.FLags) error {
	if flags.MemtableMaxSize != nil {
		c.Memtable.MemtableMaxSize = *flags.MemtableMaxSize
	}
	if flags.MemtableMaxSizeKb != nil {
		c.Memtable.MemtableSizeKB = *flags.MemtableMaxSizeKb
	}
	if flags.MemtableType != nil {
		c.Memtable.MemtableType = *flags.MemtableType
	}
	if flags.WalSegmentSize != nil {
		c.WAL.SegmentSize = *flags.WalSegmentSize
	}
	return nil
}

func (c *Config) validateFields() error {
	if c.Memtable.MemtableMaxSize <= 0 &&
		c.Memtable.MemtableSizeKB <= 0 {
		return fmt.Errorf("memtable size must be positive")
	}

	if c.Memtable.MemtableType != "hashmap" &&
		c.Memtable.MemtableType != "skiplist" &&
		c.Memtable.MemtableType != "btree" {
		return fmt.Errorf("invalid memtable type")
	}

	if c.WAL.SegmentSize <= 0 {
		return fmt.Errorf("segment size must be positive")
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
