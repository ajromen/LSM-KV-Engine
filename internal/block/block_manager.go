package block

import (
	"encoding/json"
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
	FilePath string
	Offset   uint32 // ako je blok 1kb dozvoljava segment size od 4 terabajta (16 bitova daje max 64mb)
}

func NewBlockManager(blockSize, maxLRUSize int) *BlockManager {
	return &BlockManager{
		blockSize: blockSize,
		cache:     cache.NewLRU[BlockKey, []byte](maxLRUSize),
	}
}

// BlockSize Getter (WAL needs to validate cfg.BlockSize == bm.BlockSize())
func (bm *BlockManager) BlockSize() int {
	return bm.blockSize
}

func (bm *BlockManager) Read(key BlockKey) ([]byte, error) {
	if val, ok := bm.cache.Get(key); ok {
		return val, nil
	}

	data, err := bm.ReadNoCache(key)
	if err != nil {
		return nil, err
	}

	bm.cache.Put(key, data)
	return data, nil
}

func (bm *BlockManager) ReadNoCache(key BlockKey) ([]byte, error) {
	f, err := os.Open(key.FilePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	val := make([]byte, bm.blockSize)
	_, err = f.ReadAt(val, int64(bm.blockSize)*int64(key.Offset))
	if err != nil && err != io.EOF {
		// io.EOF might be partial at the end
		// WAL reads whole blocks, eof shouldnt happen
		return nil, err
	}
	cpy := make([]byte, len(val))
	copy(cpy, val)

	return cpy, nil
}

func (bm *BlockManager) Write(key BlockKey, value []byte) error {
	if len(value) != bm.blockSize {
		return fmt.Errorf("invalid block size")
	}

	err := bm.WriteNoCache(key, value)
	if err != nil {
		return err
	}

	cpy := make([]byte, len(value))
	copy(cpy, value)
	bm.cache.Put(key, cpy)
	return nil
}

func (bm *BlockManager) WriteNoCache(key BlockKey, value []byte) error {
	f, err := os.OpenFile(key.FilePath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	offset := int64(bm.blockSize) * int64(key.Offset)
	if _, err = f.WriteAt(value, offset); err != nil {
		return err
	}

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

// WriteAt Sluzi za dopisivanje bloka na kraj fajla (ako mu se prosledi pogresan Offset u BlockKey-u nece biti zapisano na kraju)
func (bm *BlockManager) WriteAt(f *os.File, key BlockKey, value []byte) error {
	if len(value) != bm.blockSize {
		return fmt.Errorf("invalid block size")
	}

	err := bm.WriteAtNoCache(f, key, value)
	if err != nil {
		return err
	}

	bm.cache.Put(key, value)
	return nil
}

func (bm *BlockManager) WriteAtNoCache(f *os.File, key BlockKey, value []byte) error {
	offset := int64(bm.blockSize) * int64(key.Offset)
	if _, err := f.WriteAt(value, offset); err != nil {
		return err
	}
	return nil
}

func NewBlockKey(filePath string, offset uint32) BlockKey {
	return BlockKey{FilePath: filePath, Offset: offset}
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

// WriteNoBlock used for specific parts of database
func WriteNoBlock(f *os.File, offset uint64, data []byte) error {
	_, err := f.Seek(int64(offset), io.SeekStart)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err != nil {
		return err
	}
	return nil
}

func ReadNoBlock(f *os.File, offset uint64, size uint32) ([]byte, error) {
	data := make([]byte, size)
	if _, err := f.ReadAt(data, int64(offset)); err != nil {
		return nil, err
	}
	return data, nil
}

func ReadJSON(filePath string, obj any) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, obj); err != nil {
		return err
	}
	return nil
}

func WriteJSON(filePath string, obj any) error {
	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return err
	}

	return nil
}

func EnsureDir(path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}
	return nil
}

func DeleteFile(path string) error {
	err := os.Remove(path)
	if err != nil {
		return err
	}
	return nil
}
