package memtable

import "github.com/ajromen/LSM-KV-Engine/internal/iterator"

// this file contains all interfaces and abstractions of elements in core of a memtable

// MemtableEntry is a single key-value record in a memtable with versioning timestamp and tombstone mark
type MemtableEntry struct {
	Key       []byte
	Value     []byte
	Timestamp uint64
	Tombstone bool
}

// HashKey converts the binary key into a string nnd is used so hash-based structures can be generically implemented
func (m MemtableEntry) HashKey() string {
	return string(m.Key)
}

// MemtableStore defines the interface for underlying in-memory data structures
type MemtableStore interface {
	Insert(entry MemtableEntry)
	Search(entry MemtableEntry) *MemtableEntry
	EntriesInOrder() []MemtableEntry
	Reset()
	Size() int
	Visualize(func(MemtableEntry) string) string
	RawIterator() iterator.Iterator[MemtableEntry]
	Iterator() iterator.Iterator[MemtableEntry]
}

// Memtable defines the high-level behavior of a memtable.
type Memtable interface {
	Put(key []byte, value []byte, timeStamp uint64, tombstone bool)
	Get(key []byte) ([]byte, bool)
	Delete(key []byte, timeStamp uint64)
	ShouldFlush() bool
	Reset()
	Flush() []MemtableEntry
	ReadEntries() []MemtableEntry
	NumEntries() int
	SizeBytes() uint64
	Visualize() string
	RawIterator() iterator.Iterator[MemtableEntry]
	Iterator() iterator.Iterator[MemtableEntry]
}

// GenericMemtable is a concrete implementation of Memtable.
type GenericMemtable struct {
	store         MemtableStore
	numEntries    int
	sizeBytes     uint64
	maxNumEntries int
	maxSizeBytes  uint64

	flushHandler func([]MemtableEntry)
}
