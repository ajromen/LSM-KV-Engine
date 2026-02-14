package memtable

import "math/rand"

type Node struct {
	entry MemtableEntry
	next  []*Node
}

type SkipList struct {
	header   *Node
	level    int
	size     int
	MaxLevel int
}

func NewNode(entry MemtableEntry, level int) *Node {
	return &Node{
		entry: entry,
		next:  make([]*Node, level),
	}
}

func NewSkipList(MaxLevel int) *SkipList {
	headerEntry := MemtableEntry{}
	return &SkipList{
		header:   NewNode(headerEntry, MaxLevel),
		level:    1,
		size:     0,
		MaxLevel: MaxLevel,
	}
}

func (sl *SkipList) findRandLevel(MaxLevel int) int {
	level := 1
	for rand.Float32() < 0.5 && level < MaxLevel {
		level++
	}
	return level
}

func (sl *SkipList) Insert(entry MemtableEntry) {
	update := make([]*Node, sl.MaxLevel)
	current := sl.header
	for i := sl.level - 1; i >= 0; i-- {
		for current.next[i] != nil && current.next[i].entry.Key < entry.Key {
			current = current.next[i]
		}
		update[i] = current
	}
	current = current.next[0]
	if current != nil && current.entry.Key == entry.Key {
		current.entry = entry
	} else {
		randLevel := sl.findRandLevel(sl.MaxLevel)
		if randLevel > sl.level {
			for i := sl.level; i < randLevel; i++ {
				update[i] = sl.header
			}
			sl.level = randLevel
		}
		newNode := NewNode(entry, randLevel)
		for i := 0; i < randLevel; i++ {
			newNode.next[i] = update[i].next[i]
			update[i].next[i] = newNode
		}
		sl.size++
	}
}

func (sl *SkipList) MarkDeleted(key string) {
	entry := MemtableEntry{
		Key:       key,
		Value:     nil,
		Tombstone: true,
	}
	sl.Insert(entry)
}

func (sl *SkipList) Search(key string) (MemtableEntry, bool) {
	current := sl.header
	for i := sl.level - 1; i >= 0; i-- {
		for current.next[i] != nil && current.next[i].entry.Key < key {
			current = current.next[i]
		}
	}
	current = current.next[0]
	if current != nil && current.entry.Key == key {
		return current.entry, true
	}
	return MemtableEntry{}, false
}

func NewSkipListMem(maxSize int, flushHandler func([]MemtableEntry)) *SkipListMemtable {
	return &SkipListMemtable{
		memtableData: NewSkipList(16),
		maxSize:      maxSize,
		flushHandler: flushHandler,
	}
}

func (memtable *SkipListMemtable) Put(key string, value []byte) {
	shouldFlush := memtable.Flush()
	if shouldFlush {
		entries := memtable.FlushEntries()
		if memtable.flushHandler != nil {
			memtable.flushHandler(entries)
		}
	}
	entry := MemtableEntry{
		Key:       key,
		Value:     value,
		Tombstone: false,
	}
	memtable.memtableData.Insert(entry)
}

func (memtable *SkipListMemtable) Get(key string) (MemtableEntry, bool) {
	return memtable.memtableData.Search(key)
}

func (memtable *SkipListMemtable) Delete(key string) {
	memtable.memtableData.MarkDeleted(key)
}

func (memtable *SkipListMemtable) Flush() bool {
	return memtable.memtableData.size >= memtable.maxSize
}

func (memtable *SkipListMemtable) Reset() {
	memtable.memtableData = NewSkipList(16)
}

func (memtable *SkipListMemtable) FlushEntries() []MemtableEntry {
	entries := make([]MemtableEntry, 0, memtable.memtableData.size)
	current := memtable.memtableData.header.next[0]
	for current != nil {
		entries = append(entries, current.entry)
		current = current.next[0]
	}
	memtable.Reset()
	return entries
}

func (memtable *SkipListMemtable) ReadEntriesNoFlushing() []MemtableEntry {
	entries := make([]MemtableEntry, 0, memtable.memtableData.size)
	current := memtable.memtableData.header.next[0]
	for current != nil {
		entries = append(entries, current.entry)
		current = current.next[0]
	}
	return entries
}

func (memtable *SkipListMemtable) Size() int {
	return memtable.memtableData.size
}
