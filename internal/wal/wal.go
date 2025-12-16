package wal

import "github.com/ajromen/LSM-KV-Engine/internal/config"

type WAL struct {
	config *config.WALConfig
}

func NewWAL(config *config.WALConfig) *WAL {
	return &WAL{config: config}
}
