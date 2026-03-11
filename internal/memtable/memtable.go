package memtable

import (
	"bytes"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
)

func NewMemtable(t MemtableType, maxNumEntries int, maxSizeBytes uint64, cfg config.MemtableConfig) Memtable {
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
	switch t {
	case TypeBTree:
		degree := cfg.BTreeConfig.MinimumDegree
		if degree == 0 {
			degree = 3
		}
		store = NewBTreeStore(degree, cmp)
	case TypeSkipList:
		level := cfg.SkipListConfig.MaxLevel
		if level == 0 {
			level = 16
		}
		store = NewSkipListStore(level, cmp)
	case TypeHashMap:
		store = NewHashMapStore()
	default:
		panic("unknown memtable type: " + t)
	}
	return NewGenericMemtable(store, maxNumEntries, maxSizeBytes)
}

func NewFactory(t MemtableType, maxNumEntries int, maxSizeBytes uint64, cfg config.MemtableConfig) func() Memtable {
	return func() Memtable {
		return NewMemtable(t, maxNumEntries, maxSizeBytes, cfg)
	}
}
