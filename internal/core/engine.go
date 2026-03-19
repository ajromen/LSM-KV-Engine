package core

import (
	"fmt"
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
	if err != nil {
		return nil, err
	}
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

func (engine *Engine) DataRaw() {
	files := []string{
		"/home/ajromen/.local/share/lsm-kv-engine/000000.sst.data",
		"/home/ajromen/.local/share/lsm-kv-engine/000001.sst.data",
		"/home/ajromen/.local/share/lsm-kv-engine/000002.sst.data",
		"/home/ajromen/.local/share/lsm-kv-engine/000003.sst.data",
	}

	for _, filePath := range files {
		dataF, err := os.Open(filePath)
		if err != nil {
			fmt.Println("open error:", filePath, err)
			continue
		}

		stats, err := dataF.Stat()
		if err != nil {
			fmt.Println("stat error:", filePath, err)
			dataF.Close()
			continue
		}

		buf := make([]byte, int(stats.Size()))

		n, err := dataF.ReadAt(buf, 0)
		if err != nil {
			fmt.Println("read error:", filePath, err)
			dataF.Close()
			continue
		}

		buf = buf[:n]

		fmt.Println("File:", filePath)
		fmt.Println("Data bytes:", buf)
		fmt.Println("-----")

		dataF.Close()
	}
}
