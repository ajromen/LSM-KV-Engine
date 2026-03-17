package memtable

import (
	"bytes"
	"math"

	"github.com/ajromen/LSM-KV-Engine/internal/iterator"
)

func NewGenericMemtable(store MemtableStore, maxNumEntries int, maxSizeBytes uint64) *GenericMemtable {
	return &GenericMemtable{
		store:         store,
		maxNumEntries: maxNumEntries,
		maxSizeBytes:  maxSizeBytes,
	}
}

func (m *GenericMemtable) Put(key []byte, value []byte, timeStamp uint64, tombstone bool) {
	sizeEntry := len(key) + len(value) + 8 + 1
	m.store.Insert(MemtableEntry{
		Key:       key,
		Value:     value,
		Timestamp: timeStamp,
		Tombstone: tombstone,
	})
	m.numEntries++
	m.sizeBytes += uint64(sizeEntry)
}

func (m *GenericMemtable) Get(key []byte) ([]byte, bool) {
	dummy := MemtableEntry{
		Key:       key,
		Timestamp: math.MaxUint64,
	}
	entry := m.store.Search(dummy)
	if entry == nil {
		return nil, false
	}
	if !bytes.Equal(entry.Key, key) {
		return nil, false
	}
	if entry.Tombstone {
		return nil, true
	}
	return entry.Value, true
}

func (m *GenericMemtable) Delete(key []byte, timestamp uint64) {
	m.Put(key, nil, timestamp, true)
}

func (m *GenericMemtable) ShouldFlush() bool {
	return m.numEntries >= m.maxNumEntries
}

func (m *GenericMemtable) Reset() {
	m.store.Reset()
	m.numEntries = 0
	m.sizeBytes = 0
}

func (m *GenericMemtable) Flush() []MemtableEntry {
	entries := m.store.EntriesInOrder()
	m.Reset()
	return entries
}

func (m *GenericMemtable) ReadEntries() []MemtableEntry {
	return m.store.EntriesInOrder()
}

func (m *GenericMemtable) NumEntries() int   { return m.numEntries }
func (m *GenericMemtable) SizeBytes() uint64 { return m.sizeBytes }

func (m *GenericMemtable) RawIterator() iterator.Iterator[MemtableEntry] {
	return m.store.RawIterator()
}
func (m *GenericMemtable) Iterator() iterator.Iterator[MemtableEntry] {
	return m.store.Iterator()
}

func (m *GenericMemtable) Visualize() string {
	return m.store.Visualize(func(e MemtableEntry) string {
		return string(e.Key)
	})
}
