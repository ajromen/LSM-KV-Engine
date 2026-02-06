package sstable

import (
	"errors"
	"fmt"
	"os"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
)

type SSTableWriter struct {
	filePath     string
	file         *os.File
	blockManager *block.BlockManager
	blockBuilder *DataBlockBuilder
	blockOffset  uint32
	closed       bool
	compression  byte
}

func NewSSTableWriter(filePath string, blockManager *block.BlockManager, restartInterval int, compression byte) (*SSTableWriter, error) {
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}
	return &SSTableWriter{
		filePath:     filePath,
		file:         file,
		blockManager: blockManager,
		blockBuilder: NewDataBlockBuilder(restartInterval, blockManager.BlockSize()),
		blockOffset:  0,
		compression:  compression,
	}, nil
}

func (sw *SSTableWriter) Add(record Record) error {
	if !sw.blockBuilder.AddRecord(record) {
		if err := sw.Flush(); err != nil {
			return errors.New("Failed to flush")
		}
		if !sw.blockBuilder.AddRecord(record) {
			return errors.New("Failed to add record")
		}
	}
	return nil
}

func (sw *SSTableWriter) Flush() error {
	if sw.file == nil {
		return nil
	}
	if sw.blockBuilder.recordCount == 0 {
		return errors.New("no records to write")
	}
	blockData := sw.blockBuilder.Finish(sw.compression)
	blockKey := block.BlockKey{
		FilePath: sw.filePath,
		Offset:   sw.blockOffset,
	}
	if err := sw.blockManager.WriteAt(sw.file, blockKey, blockData); err != nil {
		return err
	}
	sw.blockOffset++
	sw.blockBuilder.Reset()
	return nil
}

func (sw *SSTableWriter) Close() error {
	if sw.closed {
		return nil
	}
	if err := sw.Flush(); err != nil {
		return fmt.Errorf("failed to flush final block: %w", err)
	}
	if err := sw.file.Close(); err != nil {
		return fmt.Errorf("failed to close file: %w", err)
	}
	sw.closed = true
	return nil
}
