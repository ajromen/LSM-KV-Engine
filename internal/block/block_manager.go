package block

import (
	"fmt"
	"os"

	"github.com/ajromen/LSM-KV-Engine/internal/cache"
)

type BlockManager struct {
	blockSize int
	cache     *cache.LRU[BlockKey, []byte]
}

type BlockKey struct {
	filePath string
	offset   uint32 // ako je blok 1kb dozvoljava segment size od 4 terabajta (16 bitova daje max 64mb)
}

func NewBlockManager(blockSize, maxLRUSize int) *BlockManager {
	return &BlockManager{
		blockSize: blockSize,
		cache:     cache.NewLRU[BlockKey, []byte](maxLRUSize),
	}
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

	if _, err = f.ReadAt(val, int64(bm.blockSize)*int64(key.offset)); err != nil {
		return nil, err
	}

	bm.cache.Put(key, val)

	return val, nil
}

// Koristiti ako fajl nije vec otvoren
func (bm *BlockManager) Write(key BlockKey, value []byte) error {
	if len(value) != bm.blockSize {
		return fmt.Errorf("invalid block size")
	}

	f, err := os.OpenFile(key.filePath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	offset := int64(bm.blockSize) * int64(key.offset)

	if _, err = f.WriteAt(value, offset); err != nil {
		return err
	}

	bm.cache.Put(key, value)

	return nil
}

// WriteAt Sluzi za dopisivanje bloka na kraj fajla (ako mu se prosledi pogresan offset u BlockKey-u nece biti zapisano na kraju)
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
