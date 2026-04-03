package memtable

import (
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
	"github.com/ajromen/LSM-KV-Engine/internal/iterator"
)

// this file contains all interfaces and abstractions of elements in core of a memtable

// MemtableEntry is a single key-value record in a memtable with versioning sequenceId and operation type
// different operation types -> put, delete, merge, delete range
// case put: opType=0       | key          | value          | seqId | expiresAt
// case del: opType=1       | key          | nil            | seqId | nil
// case merge: opType=2     | key          | value=delta_op | seqId | expiresAt(maybe?)
// case range del: opType=3 | key=startKey | value=endKey   | seqId | nil
type MemtableEntry struct {
	OpType enums.OpType // type of operation being saved -> older version had tombstone now it is OpType=opTypeDelete=1

	Key   []byte // Raw key (in case of opTypeRangeDel = start key -> lower bound of given range)
	Value []byte // Raw value (in case of opTypeDelete = nil | in case of opTypeMerge = deltaOperation (+1 for example) | in case of opTypeRangeDel = endKey)

	SeqId     uint64 // sequence number -> every nonatomic operation has its own sequence number
	ExpiresAt int64  // timestamp when key expires ( timestamp(now) + given ttl)
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
	Put(key []byte, value []byte, seqId uint64, opType enums.OpType)
	PutWithTTL(key []byte, value []byte, seqId uint64, opType enums.OpType, ttl int64)
	Get(key []byte) (*MemtableEntry, bool)
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
	store MemtableStore
	rangeDelStore MemtableStore
	numEntries    int
	sizeBytes     uint64
	maxNumEntries int
	maxSizeBytes  uint64

	flushHandler func([]MemtableEntry)
}
