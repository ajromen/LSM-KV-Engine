package core

import (
	"fmt"
	"io"
	"os"

	"github.com/ajromen/LSM-KV-Engine/internal/cli"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/lsm"
	"github.com/ajromen/LSM-KV-Engine/internal/sequence"
	"github.com/ajromen/LSM-KV-Engine/internal/sstable"
)

type Engine struct {
	config *config.Config
	lsm    *lsm.LSM
	seqGen *sequence.SequenceGenerator
	//wal
}

func NewEngine(flags *cli.FLags) (*Engine, error) {
	err := config.LoadConfig(flags)
	if err != nil {
		return nil, err
	}
	dataDir := config.GetSettings().SavePath
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	lsmTree, err := lsm.NewLSM(dataDir)
	if err != nil {
		return nil, err
	}
	engine := Engine{lsm: lsmTree}
	engine.recover()
	return &engine, nil
}

// check manifest
// check wal
func (engine *Engine) recover() {
	var maxSeq uint64
	maxSeq = engine.lsm.GetMaxSeqId()
	//wal
	engine.seqGen = sequence.NewSequenceGenerator(maxSeq)
}

func (engine *Engine) Put(key []byte, value []byte) error {
	seqId := engine.seqGen.Next()
	//wal
	err := engine.lsm.Put(key, value, seqId)
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
	seqId := engine.seqGen.Next()
	// wal
	err := engine.lsm.Delete(key, seqId)
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

func (engine *Engine) DataRaw(index int) {
	basePath := fmt.Sprintf(
		"/home/rikic/.local/share/lsm-kv-engine/%06d.sst",
		index,
	)
	dataPath := basePath + ".data"
	var filePath string
	var readSize int64
	if _, err := os.Stat(dataPath); err == nil {
		filePath = dataPath

		info, err := os.Stat(dataPath)
		if err != nil {
			fmt.Println("stat error:", err)
			return
		}
		readSize = info.Size()

	} else {
		filePath = basePath

		f, err := os.Open(filePath)
		if err != nil {
			fmt.Println("open error:", err)
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			fmt.Println("stat error:", err)
			return
		}
		footerBuf := make([]byte, sstable.FooterSize)
		_, err = f.ReadAt(footerBuf, info.Size()-sstable.FooterSize)
		if err != nil {
			fmt.Println("footer read error:", err)
			return
		}
		footer := &sstable.Footer{}
		footer.Decode(footerBuf)
		readSize = int64(footer.FilterHandler.Offset)
	}
	f, err := os.Open(filePath)
	if err != nil {
		fmt.Println("open error:", err)
		return
	}
	defer f.Close()
	buf := make([]byte, readSize)
	n, err := f.ReadAt(buf, 0)
	if err != nil && err != io.EOF {
		fmt.Println("read error:", err)
		return
	}
	buf = buf[:n]
	fmt.Println("File:", filePath)
	fmt.Println("Data bytes:", buf)
	fmt.Println("-----")
}
