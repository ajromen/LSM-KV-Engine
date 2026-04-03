package block

import (
	"github.com/ajromen/LSM-KV-Engine/internal/cache"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
)

var instance *BlockCache

type BlockCache struct {
	lru *cache.LRU[BlockKey, []byte]
}

// ensures single block cache instance
func GetBlockCacheInstance() *BlockCache {
	if instance == nil {
		instance = &BlockCache{
			cache.NewLRU[BlockKey, []byte](config.GetSettings().BlockManager.BlockCacheMaxBlocks),
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

func (b *BlockCache) Clear() {
	if b == nil || b.lru == nil {
		return
	}
	b.lru.Clear()
}
