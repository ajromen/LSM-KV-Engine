package memtable

import "github.com/ajromen/LSM-KV-Engine/internal/config"

func NewMemtables(cfg config.Config, flushHandler func([]MemtableEntry)) *Memtables {
	instances := cfg.Memtable.Instances
	maxSize := cfg.Memtable.MemtableMaxSize
	t := cfg.Memtable.BTreeConfig.MinimumDegree
	maxLevel := cfg.Memtable.SkipListConfig.MaxLevel
	memType := cfg.Memtable.MemtableType
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
			m = NewSkipListMem(maxSize, maxLevel, flushHandler)
		case "btree":
			m = NewBTreeMem(maxSize, t, flushHandler)
		default:
			continue
		}
		if m != nil {
			tables = append(tables, m)
		}
	}
	return &Memtables{
		activeIndex:  0,
		oldestIndex:  0,
		maxTables:    instances,
		tables:       tables,
		flushHandler: flushHandler,
	}
}

func (m *Memtables) Put(key string, value []byte) {
	active := m.tables[m.activeIndex]
	active.Put(key, value)
	if active.Flush() {
		next := (m.activeIndex + 1) % m.maxTables
		if m.tables[next].Size() == 0 {
			m.activeIndex = next
			return
		}
		m.flushOldestAndRotate()
	}
}

func (m *Memtables) flushOldestAndRotate() {
	oldest := m.tables[m.oldestIndex]
	entries := oldest.FlushEntries()
	if m.flushHandler != nil {
		m.flushHandler(entries)
	}
	oldest.Reset()
	m.oldestIndex = (m.oldestIndex + 1) % len(m.tables)
	m.activeIndex = (m.activeIndex + 1) % len(m.tables)
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
	for i := 0; i < m.maxTables; i++ {
		idx := (m.activeIndex - i + m.maxTables) % m.maxTables
		entries := m.tables[idx].ReadEntriesNoFlushing()
		all = append(all, entries...)
	}
	return all
}
