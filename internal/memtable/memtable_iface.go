package memtable

import "github.com/ajromen/LSM-KV-Engine/internal/data_structures"

type MemtableEntry struct {
	Key       string
	Value     []byte
	Tombstone bool
}

type Memtable interface {
	Put(key string, value []byte)
	Get(key string) (MemtableEntry, bool)
	Delete(key string)
	Flush() bool
	Reset()
	FlushEntries() []MemtableEntry
	ReadEntriesNoFlushing() []MemtableEntry
	Size() int
}

type HashMapMemtable struct {
	memtableData map[string]MemtableEntry
	maxSize      int
	flushHandler func([]MemtableEntry)
}

type BTreeMemtable struct {
	memtableData *data_structures.BTree[MemtableEntry]
	maxSize      int
	flushHandler func([]MemtableEntry)
}

type SkipListMemtable struct {
	memtableData *data_structures.SkipList[MemtableEntry]
	maxSize      int
	flushHandler func([]MemtableEntry)
}

type Memtables struct {
	activeIndex  int
	oldestIndex  int
	maxTables    int
	tables       []Memtable
	flushHandler func([]MemtableEntry)
}
