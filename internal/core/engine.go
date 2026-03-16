package core

import (
	"os"

	"github.com/ajromen/LSM-KV-Engine/internal/cli"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/lsm"
)

type Engine struct {
	config *config.Config
	lsm    *lsm.LSM
	//wal
}

func NewEngine(flags *cli.FLags) (*Engine, error) {
	cfg, err := config.LoadConfig(flags)
	if err != nil {
		return nil, err
	}
	dataDir := cfg.SavePath
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	lsmTree, err := lsm.NewLSM(cfg, dataDir)
	engine := Engine{config: cfg, lsm: lsmTree}
	return &engine, nil
}

func (engine *Engine) Put(key []byte, value []byte) error {
	//wal
	err := engine.lsm.Put(key, value)
	if err != nil {
		return err
	}
	return nil
}

func (engine *Engine) Get(key []byte) ([]byte, bool, error) {
	value, found, err := engine.lsm.Get(key)
	return value, found, err
}

func (engine *Engine) Delete(key []byte) error {
	// wal
	err := engine.lsm.Delete(key)
	if err != nil {
		return err
	}
	return nil
}

func (engine *Engine) Close() error {
	//wal finish write
	err := engine.lsm.Finish()
	if err != nil {
		return err
	}
	return nil
}

func (engine *Engine) ClearAll() error {
	//wal
	print("TODO delete everything")
	return nil
}
