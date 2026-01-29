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
		else if memType == "btree" {
			return newBTree(maxSize)
		}
	*/
}
