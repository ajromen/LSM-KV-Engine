package core

import (
	"github.com/ajromen/LSM-KV-Engine/internal/cli"
)

type Engine struct {
	config *Config
	//TODO dodati wal i ostale strukutre
}

func NewEngine(flags *cli.FLags) (*Engine, error) {
	config, err := LoadConfig(*flags.ConfigPath)
	if err != nil {
		return nil, err
	}

	return &Engine{config: config}, nil
}

func (engine *Engine) Close() {
}
