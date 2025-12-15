package memtable

type HashMapMemtable struct {
	memtableData map[string]MemtableEntry
	maxSize      int
}

func NewHashMap(maxSize int) *HashMapMemtable {
	return &HashMapMemtable{
		memtableData: make(map[string]MemtableEntry),
		maxSize:      maxSize,
	}
}

func (memtable *HashMapMemtable) Put(key string, value []byte) {
	memtable.memtableData[key] = MemtableEntry{Key: key, Value: value, Tombstone: false}
}

func (memtable *HashMapMemtable) Get(key string) (MemtableEntry, bool) {
	entry, ok := memtable.memtableData[key]
	if ok {
		return entry, true
	}
	return MemtableEntry{}, false
}

func (memtable *HashMapMemtable) Delete(key string) {
	memtable.memtableData[key] = MemtableEntry{Key: key, Tombstone: true}
}

func (memtable *HashMapMemtable) Flush() bool {
	return len(memtable.memtableData) >= memtable.maxSize
}

func (memtable *HashMapMemtable) Reset() {
	memtable.memtableData = make(map[string]MemtableEntry)
}
