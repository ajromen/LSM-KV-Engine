package core

import (
	"github.com/ajromen/LSM-KV-Engine/internal/cli"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/memtable"
)

type Engine struct {
	config   *config.Config
	memtable memtable.Memtable
	//TODO dodati wal i ostale strukutre
}

func NewEngine(flags *cli.FLags) (*Engine, error) {
	cfg, err := config.LoadConfig(flags)
	if err != nil {
		return nil, err
	}

	return &Engine{config: cfg}, nil
}

func (engine *Engine) Close() {
}
