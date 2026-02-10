package memtable

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
		m.flushOldestAndRotate()
	}
}

func (m *Memtables) flushOldestAndRotate() {
	oldestIndex := (m.activeIndex + 1) % m.maxTables
	oldest := m.tables[oldestIndex]
	entries := oldest.FlushEntries()
	if m.flushHandler != nil {
		m.flushHandler(entries)
	}
	oldest.Reset()
	m.activeIndex = oldestIndex
}

func (m *Memtables) Get(key string) (MemtableEntry, bool) {
	for i := 0; i < m.maxTables; i++ {
		idx := (m.activeIndex - i + m.maxTables) % m.maxTables
		entry, ok := m.tables[idx].Get(key)
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
		m.flushOldestAndRotate()
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
