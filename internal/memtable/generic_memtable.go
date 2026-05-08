package memtable

import (
	"bytes"
	"math"
	"time"

	"github.com/ajromen/LSM-KV-Engine/internal/enums"
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
func NewGenericMemtable(store MemtableStore, rangeDelStore MemtableStore, maxNumEntries int, maxSizeBytes uint64) *GenericMemtable {
	return &GenericMemtable{
		store:         store,
		rangeDelStore: rangeDelStore,
		maxNumEntries: maxNumEntries,
		maxSizeBytes:  maxSizeBytes,
	}
}

// Put adds entry to underlying data structure and updates number of entries and size of memtable in bytes
func (m *GenericMemtable) Put(key []byte, value []byte, seqId uint64, opType enums.OpType) {
	sizeEntry := len(key) + len(value) + 8 + 8 + 1
	if opType == enums.OpTypeRangeDel {
		m.rangeDelStore.Insert(MemtableEntry{
			Key:       key,
			Value:     value,
			SeqId:     seqId,
			ExpiresAt: 0,
			OpType:    opType,
		})
		return
	}
	m.store.Insert(MemtableEntry{
		Key:       key,
		Value:     value,
		SeqId:     seqId,
		ExpiresAt: 0,
		OpType:    opType,
	})
	m.numEntries++
	m.sizeBytes += uint64(sizeEntry)
}

// Upsert inserts or replaces the entry for this key (no versioning).
// numEntries only grows when a new key is written, not on overwrite.
func (m *GenericMemtable) Upsert(key []byte, value []byte, seqId uint64, opType enums.OpType) {
	entry := MemtableEntry{Key: key,
		Value:  value,
		SeqId:  seqId,
		OpType: opType,
	}
	replaced := m.store.Upsert(entry)
	if !replaced {
		m.numEntries++
		m.sizeBytes += uint64(len(key) + len(value) + 8 + 8 + 1)
	}
}

func (m *GenericMemtable) PutWithTTL(key []byte, value []byte, seqId uint64, opType enums.OpType, ttl int64) {
	expiresAt := time.Now().UnixMilli() + ttl
	sizeEntry := len(key) + len(value) + 8 + 8 + 1
	m.store.Insert(MemtableEntry{
		Key:       key,
		Value:     value,
		SeqId:     seqId,
		ExpiresAt: expiresAt,
		OpType:    opType,
	})
	m.numEntries++
	m.sizeBytes += uint64(sizeEntry)
}

func (m *GenericMemtable) UpsertWithTTL(key []byte, value []byte, seqId uint64, opType enums.OpType, ttl int64) {
	expiresAt := time.Now().UnixMilli() + ttl
	entry := MemtableEntry{
		Key: key, Value: value,
		SeqId:     seqId,
		ExpiresAt: expiresAt,
		OpType:    opType,
	}
	replaced := m.store.Upsert(entry)
	if !replaced {
		m.numEntries++
		m.sizeBytes += uint64(len(key) + len(value) + 8 + 8 + 1)
	}
}

// Remove physically deletes a specific entry from the store.
// Used for batch rollback — does not write a tombstone.
func (m *GenericMemtable) Remove(key []byte, seqId uint64) {
	entry := MemtableEntry{Key: key, SeqId: seqId}
	m.store.Remove(entry)
	m.numEntries--
	if m.numEntries < 0 {
		m.numEntries = 0
	}
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
	if m.isRangeDeleted(m.filtrateNewerRanges(m.gatherAllRangeDeletions(key), entry.SeqId), key) {
		return nil, true
	}
	if entry.OpType == enums.OpTypeDel {
		return nil, true
	}
	return entry, true
}

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
func (m *GenericMemtable) Flush() ([]MemtableEntry, []MemtableEntry) {
	entries := m.store.EntriesInOrder()
	rangeDelEntries := m.rangeDelStore.EntriesInOrder()
	m.Reset()
	return entries, rangeDelEntries
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

// Current implementation is using the same underlying data structure as memtable
// better performance -> Interval Tree -> will be implemented if we have time

func (m *GenericMemtable) gatherAllRangeDeletions(key []byte) []MemtableEntry {
	memIterator := m.rangeDelStore.Iterator()
	memIterator.SeekToFirst()
	rangeDels := make([]MemtableEntry, 0)
	for memIterator.Valid() && memIterator.Key().OpType == enums.OpTypeRangeDel {
		rangeDels = append(rangeDels, memIterator.Value())
		memIterator.Next()
	}
	return rangeDels
}

func (m *GenericMemtable) filtrateNewerRanges(ranges []MemtableEntry, seqId uint64) []MemtableEntry {
	validRanges := make([]MemtableEntry, 0)
	for _, entry := range ranges {
		if entry.SeqId > seqId {
			validRanges = append(validRanges, entry)
		}
	}
	return validRanges
}

func (m *GenericMemtable) isRangeDeleted(validRanges []MemtableEntry, key []byte) bool {
	for _, entry := range validRanges {
		if bytes.Compare(entry.Key, key) <= 0 && bytes.Compare(entry.Value, key) >= 0 {
			return true
		}
	}
	return false
}

func (m *GenericMemtable) IsCoveredByRangeDel(key []byte, keySeqId uint64) bool {
	rangeDels := m.gatherAllRangeDeletions(key)
	validRanges := m.filtrateNewerRanges(rangeDels, keySeqId)
	return m.isRangeDeleted(validRanges, key)
}
