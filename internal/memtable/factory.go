package memtable

import "github.com/ajromen/LSM-KV-Engine/internal/config"

func NewMemtable(config config.MemtableConfig) Memtable {
	if config.MemtableType == "hashmap" {
		return NewHashMap(config.MemtableMaxSize)
	} else {
		return nil
	}
	/*
		else if memType == "skiplist" {
			return NewSkipList(maxSize)
		}
		else if memType == "btree" {
			return newBTree(maxSize)
		}
	*/
}
