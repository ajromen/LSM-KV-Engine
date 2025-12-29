package memtable

func NewMemtable(memType string, maxSize int, flushHandler func([]MemtableEntry)) Memtable {
	switch memType {
	case "hashmap":
		return NewHashMap(maxSize, flushHandler)
	case "skiplist":
		return NewSkipListMem(maxSize, flushHandler)
	case "btree":
		return nil
	default:
		return nil
	}
}
