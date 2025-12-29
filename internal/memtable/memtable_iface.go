package memtable

type MemtableEntry struct {
	Key       string
	Value     []byte
	Tombstone bool
}

type Memtable interface {
	Put(key string, value []byte)
	Get(key string) (MemtableEntry, bool)
	Delete(key string) bool
	Flush() bool
	Reset()
	FlushEntries() []MemtableEntry
}
