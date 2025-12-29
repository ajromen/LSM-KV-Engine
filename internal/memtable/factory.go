package memtable

type Memtables struct {
	index     int
	lastIndex int
	maxTables int
	tables    []Memtable
}

func NewMemtable(memType string, maxSize int, flushHandler func([]MemtableEntry)) *Memtables {
	instances := 1 // ovo ce biti parametar fje
	if instances < 1 {
		instances = 1
	}
	memtables := &Memtables{
		tables:    make([]Memtable, 0, instances),
		maxTables: instances,
	}
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
			memtables.tables = append(memtables.tables, m)
		}
	}
	return memtables
}

func (memtables *Memtables) Next() Memtable {
	if len(memtables.tables) == 0 {
		return nil
	}
	memtables.index = (memtables.index + 1) % len(memtables.tables)
	return memtables.tables[memtables.index]
}
