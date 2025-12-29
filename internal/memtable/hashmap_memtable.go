package memtable

type HashMapMemtable struct {
	memtableData map[string]MemtableEntry
	maxSize      int
	flushHandler func([]MemtableEntry)
}

func NewHashMap(maxSize int, flushHandler func([]MemtableEntry)) *HashMapMemtable {
	return &HashMapMemtable{
		memtableData: make(map[string]MemtableEntry),
		maxSize:      maxSize,
		flushHandler: flushHandler,
	}
}

func (memtable *HashMapMemtable) Put(key string, value []byte) {
	shouldFlush := memtable.Flush()
	if shouldFlush {
		entries := memtable.FlushEntries()
		if memtable.flushHandler != nil {
			memtable.flushHandler(entries)
		}
	}
	memtable.memtableData[key] = MemtableEntry{Key: key, Value: value, Tombstone: false}
}

func (memtable *HashMapMemtable) Get(key string) (MemtableEntry, bool) {
	entry, ok := memtable.memtableData[key]
	if ok {
		return entry, true
	}
	return MemtableEntry{}, false
}

func (memtable *HashMapMemtable) Delete(key string) bool {
	entry, ok := memtable.memtableData[key]
	if !ok {
		return false
	}
	entry.Tombstone = true
	memtable.memtableData[key] = entry
	return true
}

func (memtable *HashMapMemtable) Flush() bool {
	return len(memtable.memtableData) >= memtable.maxSize
}

func (memtable *HashMapMemtable) Reset() {
	memtable.memtableData = make(map[string]MemtableEntry)
}

func (memtable *HashMapMemtable) FlushEntries() []MemtableEntry {
	entries := make([]MemtableEntry, 0, len(memtable.memtableData))
	keys := make([]string, 0, len(memtable.memtableData))
	for key := range memtable.memtableData {
		keys = append(keys, key)
	}
	quickSort(keys, 0, len(keys)-1)
	for _, key := range keys {
		entries = append(entries, memtable.memtableData[key])
	}
	memtable.Reset()
	return entries
}

func (memtable *HashMapMemtable) ReadEntriesNoFlushing() []MemtableEntry {
	entries := make([]MemtableEntry, 0, len(memtable.memtableData))
	keys := make([]string, 0, len(memtable.memtableData))
	for key := range memtable.memtableData {
		keys = append(keys, key)
	}
	quickSort(keys, 0, len(keys)-1)
	for _, key := range keys {
		entries = append(entries, memtable.memtableData[key])
	}
	return entries
}

func quickSort(arr []string, low int, high int) {
	if low < high {
		p := partition(arr, low, high)
		quickSort(arr, low, p-1)
		quickSort(arr, p+1, high)
	}
}

func partition(arr []string, low int, high int) int {
	pivot := arr[high]
	i := low - 1
	for j := low; j < high; j++ {
		if arr[j] < pivot {
			i++
			arr[i], arr[j] = arr[j], arr[i]
		}
	}
	arr[i+1], arr[high] = arr[high], arr[i+1]
	return i + 1
}
