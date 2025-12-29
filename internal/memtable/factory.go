package memtable

func NewMemtable(memType string, maxSize int) Memtable {
	switch memType {
	case "hashmap":
		return NewHashMap(maxSize)
	case "skiplist":
		return NewSkipListMem(maxSize)
	case "btree":
		return NewBTreeMem(maxSize)
	default:
		return nil
	}
}
