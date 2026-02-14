package block

import (
	"fmt"
	"io"
	"os"

	"github.com/ajromen/LSM-KV-Engine/internal/cache"
)

type BlockManager struct {
	blockSize int
	cache     *cache.LRU[BlockKey, []byte]
}

type BlockKey struct {
	filePath string
	offset   uint32
}

func NewBlockManager(blockSize, maxLRUSize int) *BlockManager {
	return &BlockManager{
		blockSize: blockSize,
		cache:     cache.NewLRU[BlockKey, []byte](maxLRUSize),
	}
}

// Getter (WAL needs to validate cfg.BlockSize == bm.BlockSize())
func (bm *BlockManager) BlockSize() int {
	return bm.blockSize
}

func (bm *BlockManager) Read(key BlockKey) ([]byte, error) {
	if val, ok := bm.cache.Get(key); ok {
		return val, nil
	}

	f, err := os.Open(key.filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	val := make([]byte, bm.blockSize)
	_, err = f.ReadAt(val, int64(bm.blockSize)*int64(key.offset))
	if err != nil && err != io.EOF {
		// io.EOF might be partial at the end
		// WAL reads whole blocks, eof shouldnt happen
		return nil, err
	}
	cpy := make([]byte, len(val))
	copy(cpy, val)
	bm.cache.Put(key, cpy)
	return cpy, nil
}

func (bm *BlockManager) Write(key BlockKey, value []byte) error {
	if len(value) != bm.blockSize {
		return fmt.Errorf("invalid block size")
	}

	f, err := os.OpenFile(key.filePath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	offset := int64(bm.blockSize) * int64(key.offset)
	if _, err = f.WriteAt(value, offset); err != nil {
		return err
	}

	cpy := make([]byte, len(value))
	copy(cpy, value)
	bm.cache.Put(key, cpy)
	return nil
}

func (bm *BlockManager) SyncFile(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

func (bm *BlockManager) WriteAt(f *os.File, key BlockKey, value []byte) error {
	if len(value) != bm.blockSize {
		return fmt.Errorf("invalid block size")
	}

	offset := int64(bm.blockSize) * int64(key.offset)
	if _, err := f.WriteAt(value, offset); err != nil {
		return err
	}

	bm.cache.Put(key, value)
	return nil
}

func NewBlockKey(filePath string, offset uint32) BlockKey {
	return BlockKey{filePath: filePath, offset: offset}
}

// EnsureSize for fixed segment size (in blocks)
func (bm *BlockManager) EnsureSize(path string, sizeBytes int64) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Size() >= sizeBytes {
		return nil
	}
	// Truncate file
	return f.Truncate(sizeBytes)
}
