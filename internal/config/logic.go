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
		err = cfg.validateFields()
		if err != nil {
			return nil, err
		}
	}
	err := cfg.applyFlags(flags)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func (config *Config) applyFlags(flags *cli.FLags) error { return nil }
func (config *Config) validateFields() error             { return nil }
func (config *Config) loadFromFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot open config file: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(config); err != nil {
		return fmt.Errorf("invalid config format: %w", err)
	}
	return nil
}
