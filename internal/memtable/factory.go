package memtable

func NewMemtable(memType string, maxSize int) Memtable {
	if memType == "hashmap" {
		return NewHashMap(maxSize)
	} else {
		return nil
	}
	/*
		else if memType == "skiplist" {
			return NewSkipList(maxSize)
		}
		else if memType == "btree" {
			return newBTree(maxSize)
		}
	*/
}
