package core

import (
	"fmt"
	"io"
	"os"
	"path"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/sstable"
)

func (engine *Engine) DataRaw(index int) {
	basePath := path.Join(config.GetSettings().SavePath, fmt.Sprintf("L0_%06d.sst", index))
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

func (engine *Engine) WalPrintAll() error {
	return engine.wal.PrintAll()
}
