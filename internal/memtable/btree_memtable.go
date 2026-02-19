package memtable

import (
	"strings"

	"github.com/ajromen/LSM-KV-Engine/internal/data_structures"
)

func NewBTreeMem(maxSize int, t int, flushHandler func([]MemtableEntry)) *BTreeMemtable {
	cmp := func(a, b MemtableEntry) int {
		return strings.Compare(a.Key, b.Key)
	}
	return &BTreeMemtable{
		memtableData: data_structures.NewBTree[MemtableEntry](t, cmp),
		maxSize:      maxSize,
		flushHandler: flushHandler,
	}
}

func (memtable *BTreeMemtable) Put(key string, value []byte) {
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

func (memtable *BTreeMemtable) Get(key string) (MemtableEntry, bool) {
	return memtable.memtableData.SearchTree(MemtableEntry{Key: key})
}

func (memtable *BTreeMemtable) Delete(key string) {
	entry := MemtableEntry{Key: key, Tombstone: true}
	memtable.memtableData.MarkDeleted(entry, func(e *MemtableEntry) {
		e.Tombstone = true
	})
}

func (memtable *BTreeMemtable) Flush() bool {
	return memtable.memtableData.Size() >= memtable.maxSize
}

func (memtable *BTreeMemtable) Reset() {
	cmp := func(a, b MemtableEntry) int {
		return strings.Compare(a.Key, b.Key)
	}
	memtable.memtableData = data_structures.NewBTree[MemtableEntry](8, cmp)
}

func (memtable *BTreeMemtable) FlushEntries() []MemtableEntry {
	entries := memtable.memtableData.EntriesInOrder()
	memtable.Reset()
	return entries
}

func (memtable *BTreeMemtable) ReadEntriesNoFlushing() []MemtableEntry {
	entries := memtable.memtableData.EntriesInOrder()
	return entries
}

func (memtable *BTreeMemtable) Size() int {
	return len(memtable.memtableData.EntriesInOrder())
}
