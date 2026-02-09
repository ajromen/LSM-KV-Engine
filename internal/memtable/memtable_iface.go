package memtable

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
}

type HashMapMemtable struct {
	memtableData map[string]MemtableEntry
	maxSize      int
	flushHandler func([]MemtableEntry)
}

type BTreeMemtable struct {
	memtableData *BTree
	maxSize      int
	flushHandler func([]MemtableEntry)
}

type SkipListMemtable struct {
	memtableData *SkipList
	maxSize      int
	flushHandler func([]MemtableEntry)
}

type Memtables struct {
	activeIndex  int
	maxTables    int
	tables       []Memtable
	flushHandler func([]MemtableEntry)
}
