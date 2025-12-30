package memtable

type Memtables struct {
	activeIndex  int
	maxTables    int
	tables       []Memtable
	flushHandler func([]MemtableEntry)
}

func NewMemtables(memType string, instances int, maxSize int, flushHandler func([]MemtableEntry)) *Memtables {
	if instances < 1 {
		instances = 1
	}
	tables := make([]Memtable, 0, instances)
	for i := 0; i < instances; i++ {
		var m Memtable
		switch memType {
		case "hashmap":
			m = NewHashMap(maxSize, flushHandler)
		case "skiplist":
			m = NewSkipListMem(maxSize, flushHandler)
		case "btree":
			m = NewBTreeMem(maxSize, flushHandler)
		default:
			continue
		}
		if m != nil {
			tables = append(tables, m)
		}
	}
	return &Memtables{
		activeIndex:  0,
		maxTables:    instances,
		tables:       tables,
		flushHandler: flushHandler,
	}
}

func (m *Memtables) Put(key string, value []byte) {
	active := m.tables[m.activeIndex]
	active.Put(key, value)
	if active.Flush() {
		m.rotateOrFlushAll()
	}
}

func (m *Memtables) rotateOrFlushAll() {
	if m.activeIndex < m.maxTables-1 {
		m.activeIndex++
		return
	}
	allEntries := make([]MemtableEntry, 0)
	for i := 0; i < m.maxTables; i++ {
		entries := m.tables[i].FlushEntries()
		allEntries = append(allEntries, entries...)
	}
	if m.flushHandler != nil {
		m.flushHandler(allEntries)
	}
	for i := 0; i < m.maxTables; i++ {
		m.tables[i].Reset()
	}
	m.activeIndex = 0
}

func (m *Memtables) Get(key string) (MemtableEntry, bool) {
	for i := m.activeIndex; i >= 0; i-- {
		entry, ok := m.tables[i].Get(key)
		if ok {
			if entry.Tombstone {
				return MemtableEntry{}, false
			}
			return entry, true
		}
	}
	return MemtableEntry{}, false
}

func (m *Memtables) Delete(key string) {
	active := m.tables[m.activeIndex]
	active.Delete(key)
	if active.Flush() {
		m.rotateOrFlushAll()
	}
}

func (m *Memtables) ReadEntriesNoFlushing() []MemtableEntry {
	all := make([]MemtableEntry, 0)
	for i := 0; i <= m.activeIndex; i++ {
		entries := m.tables[i].ReadEntriesNoFlushing()
		all = append(all, entries...)
	}
	return all
}
