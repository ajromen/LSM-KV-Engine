package memtable

import (
	"strings"

	"github.com/ajromen/LSM-KV-Engine/internal/data_structures"
)

func NewSkipListMem(maxSize int, maxLevel int, flushHandler func([]MemtableEntry)) *SkipListMemtable {
	cmp := func(a, b MemtableEntry) int {
		return strings.Compare(a.Key, b.Key)
	}
	return &SkipListMemtable{
		memtableData: data_structures.NewSkipList[MemtableEntry](cmp, maxLevel),
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
	return memtable.memtableData.Search(MemtableEntry{Key: key})
}

func (memtable *SkipListMemtable) Delete(key string) {
	entry := MemtableEntry{Key: key, Tombstone: true}
	memtable.memtableData.MarkDeleted(entry, func(e *MemtableEntry) {
		e.Tombstone = true
	})
}

func (memtable *SkipListMemtable) Flush() bool {
	return memtable.memtableData.Size() >= memtable.maxSize
}

func (memtable *SkipListMemtable) FlushEntries() []MemtableEntry {
	entries := memtable.memtableData.EntriesInOrder()
	memtable.memtableData.Reset()
	return entries
}

func (memtable *SkipListMemtable) ReadEntriesNoFlushing() []MemtableEntry {
	return memtable.memtableData.EntriesInOrder()
}

func (memtable *SkipListMemtable) Reset() {
	memtable.memtableData.Reset()
}

func (memtable *SkipListMemtable) Size() int {
	return memtable.memtableData.Size()
}
