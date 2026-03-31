package memtable

import (
	"bytes"
	"math"
	"time"

	"github.com/ajromen/LSM-KV-Engine/internal/iterator"
)

// Generic Memtable -> one singular instance of memtable which is generic (it itself doesn't know about inline data-structure)
// All functions use functions defined as functions that should be implemented as a MemtableStore interface
// example -> put uses MemtableStore insert which inserts a entry in underlying data structure
// additional attributes (max number of entries and max size in bytes) are used to determine whether memtable should be sent to Flush Worker
// this strucutre implements MemtableInterface

// NewGenericMemtable is called to create a singular instance of memtable and as a argument it gets store
// depending on chosen type of underlying data structure for memtable different adapters/stores are passed to it
// this allows factorization and creation of memtables to be completely generic
func NewGenericMemtable(store MemtableStore, maxNumEntries int, maxSizeBytes uint64) *GenericMemtable {
	return &GenericMemtable{
		store:         store,
		maxNumEntries: maxNumEntries,
		maxSizeBytes:  maxSizeBytes,
	}
}

// Put adds entry to underlying data structure and updates number of entries and size of memtable in bytes
func (m *GenericMemtable) Put(key []byte, value []byte, seqId uint64, tombstone bool) {
	sizeEntry := len(key) + len(value) + 8 + 8 + 1
	m.store.Insert(MemtableEntry{
		Key:       key,
		Value:     value,
		SeqId:     seqId,
		ExpiresAt: 0,
		Tombstone: tombstone,
	})
	m.numEntries++
	m.sizeBytes += uint64(sizeEntry)
}

func (m *GenericMemtable) PutWithTTL(key []byte, value []byte, seqId uint64, tombstone bool, ttl int64) {
	expiresAt := time.Now().Unix() + ttl
	sizeEntry := len(key) + len(value) + 8 + 8 + 1
	m.store.Insert(MemtableEntry{
		Key:       key,
		Value:     value,
		SeqId:     seqId,
		ExpiresAt: expiresAt,
		Tombstone: tombstone,
	})
	m.numEntries++
	m.sizeBytes += uint64(sizeEntry)
}

// Get retrieves the value for a given key
func (m *GenericMemtable) Get(key []byte) (*MemtableEntry, bool) {
	dummy := MemtableEntry{
		Key:   key,
		SeqId: math.MaxUint64,
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
	return entry, true
}

//// Delete marks a key deleted by inserting a tombstone entry
//func (m *GenericMemtable) Delete(key []byte, seqId uint64) {
//	m.Put(key, nil, seqId, true)
//}
//
//func (m *GenericMemtable) DeleteWithTTL(key []byte, seqId uint64, ttl int64) {
//	m.PutWithTTL(key, nil, seqId, true, ttl)
//}

// ShouldFlush determines whether the memtable has reached its capacity
func (m *GenericMemtable) ShouldFlush() bool {
	return m.numEntries >= m.maxNumEntries
}

// Reset clears the memtable, removing all entries and resetting counters.
func (m *GenericMemtable) Reset() {
	m.store.Reset()
	m.numEntries = 0
	m.sizeBytes = 0
}

// Flush returns all entries in sorted order and then resets the memtable.
func (m *GenericMemtable) Flush() []MemtableEntry {
	entries := m.store.EntriesInOrder()
	m.Reset()
	return entries
}

// ReadEntries returns all entries currently stored in the memtable in order.
func (m *GenericMemtable) ReadEntries() []MemtableEntry {
	return m.store.EntriesInOrder()
}

// NumEntries returns the number of entries currently stored in the memtable.
func (m *GenericMemtable) NumEntries() int { return m.numEntries }

// SizeBytes returns the total approximate size of all entries in the memtable in bytes.
func (m *GenericMemtable) SizeBytes() uint64 { return m.sizeBytes }

// RawIterator returns a low-level iterator over the entries in the memtable.
func (m *GenericMemtable) RawIterator() iterator.Iterator[MemtableEntry] {
	return m.store.RawIterator()
}

// Iterator returns a higher-level iterator over the entries in the memtable,
func (m *GenericMemtable) Iterator() iterator.Iterator[MemtableEntry] {
	return m.store.Iterator()
}

// Visualize returns a string representation of the memtable *DEBUG ONLY*.
func (m *GenericMemtable) Visualize() string {
	return m.store.Visualize(func(e MemtableEntry) string {
		return string(e.Key)
	})
}
