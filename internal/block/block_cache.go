package block

import "github.com/ajromen/LSM-KV-Engine/internal/cache"

var instance *BlockCache

type BlockCache struct {
	lru *cache.LRU[BlockKey, []byte]
}

// ensures single block cache instance
func GetBlockCacheInstance(maxLRUSize int) *BlockCache {
	if instance == nil {
		instance = &BlockCache{
			cache.NewLRU[BlockKey, []byte](maxLRUSize),
		}
	}
	return instance
}

func (b *BlockCache) Get(bk BlockKey) ([]byte, bool) {
	if val, ok := b.lru.Get(bk); ok {
		return val, true
	}
	return nil, false
}

func (b *BlockCache) Put(key BlockKey, data []byte) {
	b.lru.Put(key, data)
}
