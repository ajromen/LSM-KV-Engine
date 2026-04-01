package memtable

import (
	"bytes"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
)

// NewMemtable creates a singular instance of memtable by using NewGenericMemtable and passing a corresponding memtable store
// memtable store is adapter for underlying data structure and it allows memtable to be generic about its type
// currently implemented types of memtable based upon the underlying data structures are:
// 1. HashMapMemtable (uses HashMap which as keys has keys and as values stack of values of same keys with different values)
// 2. SkipListMemtable (uses SkipList)
// 3. BTreeMemtable (uses BTree)
// 4. RBTreeMemtable (uses Red-Black Tree)
// 5. AVLTreeMemtable (uses AVL Tree)
func NewMemtable(cfg config.MemtableConfig) Memtable {
	// function for comparing two entries -> first compare keys and if they are equal higher sequence id wins
	cmp := func(a, b MemtableEntry) int {
		if c := bytes.Compare(a.Key, b.Key); c != 0 {
			return c
		}
		if a.SeqId > b.SeqId {
			return -1
		}
		if a.SeqId < b.SeqId {
			return 1
		}
		return 0
	}

	// function for comparing two entries but it ignores seqId -> used for RBTree and AVLTree because of characteristics of these data-structs
	cmpIgnoringSeqId := func(a, b MemtableEntry) int {
		return bytes.Compare(a.Key, b.Key)
	}

	var store MemtableStore
	var rangeDelStore MemtableStore
	t := cfg.MemtableType
	switch t {
	case enums.BTreeMemTable:
		degree := cfg.BTreeConfig.MinimumDegree
		if degree == 0 {
			degree = 3
		}
		store = NewBTreeStore(degree, cmp)
		rangeDelStore = NewBTreeStore(degree, cmp)
	case enums.SkiplistMemTable:
		level := cfg.SkipListConfig.MaxLevel
		if level == 0 {
			level = 16
		}
		store = NewSkipListStore(level, cmp)
		rangeDelStore = NewSkipListStore(level, cmp)
	case enums.HashMapMemTable:
		store = NewHashMapStore()
		rangeDelStore = NewHashMapStore()
	case enums.RBTreeMemTable:
		store = NewRBTreeStore(cmp, cmpIgnoringSeqId)
		rangeDelStore = NewRBTreeStore(cmp, cmpIgnoringSeqId)
	case enums.AVLTreeMemTable:
		store = NewAVLTreeStore(cmp, cmpIgnoringSeqId)
		rangeDelStore = NewAVLTreeStore(cmp, cmpIgnoringSeqId)
	}
	return NewGenericMemtable(store, rangeDelStore, cfg.MemtableMaxEntries, cfg.MemtableMaxSizeBytes)
}

// NewFactory is used to generate memtables inside memtable manager (callback)
func NewFactory(cfg config.MemtableConfig) func() Memtable {
	return func() Memtable {
		return NewMemtable(cfg)
	}
}
