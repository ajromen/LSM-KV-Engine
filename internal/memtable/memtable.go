package memtable

import (
	"bytes"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
)

func NewMemtable(cfg config.MemtableConfig) Memtable {
	cmp := func(a, b MemtableEntry) int {
		if c := bytes.Compare(a.Key, b.Key); c != 0 {
			return c
		}
		if a.Timestamp > b.Timestamp {
			return -1
		}
		if a.Timestamp < b.Timestamp {
			return 1
		}
		return 0
	}
	//cmpIgnoringTimestamp := func(a, b MemtableEntry) int {
	//	c := bytes.Compare(a.Key, b.Key)
	//	if c != 0 {
	//		return c
	//	} else {
	//		return 0
	//	}
	//}
	var store MemtableStore
	t := cfg.MemtableType
	switch t {
	case "btree":
		degree := cfg.BTreeConfig.MinimumDegree
		if degree == 0 {
			degree = 3
		}
		store = NewBTreeStore(degree, cmp)
	case "skiplist":
		level := cfg.SkipListConfig.MaxLevel
		if level == 0 {
			level = 16
		}
		store = NewSkipListStore(level, cmp)
	case "hashmap":
		store = NewHashMapStore()
	case "rbtree":
		store = NewRBTreeStore(cmp, cmp)
	case "avltree":
		store = NewAVLTreeStore(cmp, cmp)
	default:
		panic("unknown memtable type: " + t)
	}
	return NewGenericMemtable(store, cfg.MemtableMaxEntries, cfg.MemtableMaxSizeBytes)
}

func NewFactory(cfg config.MemtableConfig) func() Memtable {
	return func() Memtable {
		return NewMemtable(cfg)
	}
}
