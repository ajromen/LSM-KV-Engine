package memtable

func newMemtable(memType string, maxSize int) Memtable {
	if memType == "hashmap" {
		return NewHashMap(maxSize)
	}
	return nil
}
