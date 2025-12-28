package block

import "github.com/ajromen/LSM-KV-Engine/internal/cache"

type BlockManager struct {
	blockSize int
	cache     *cache.LRU
}

type BlockKey struct {
	filePath string
	offset   uint32 // ako je blok 1kb dozvoljava segment size od 4 terabajta (16 bitova daje max 64mb)
}

func NewBlockManager(blockSize int) *BlockManager {
	return &BlockManager{blockSize: blockSize}
}

//type BlockMenager interface {
//	GetBlock(key BlockKey)
//	WriteBlock(key BlockKey, value interface{})
//}
