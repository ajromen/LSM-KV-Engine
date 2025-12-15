package memtable

func NewMemtable(memType string, maxSize int) Memtable {
	if memType == "hashmap" {
		return NewHashMap(maxSize)
	} else if memType == "skiplist" {
		return NewSkipListMem(maxSize)
	} else {
		return nil
	}
	/*

		else if memType == "btree" {
			return newBTree(maxSize)
		}
	*/
}
