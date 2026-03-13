package memtable

import "github.com/ajromen/LSM-KV-Engine/internal/iterator"

type MemtableEntry struct {
	Key       []byte
	Value     []byte
	Timestamp uint64
	Tombstone bool
}

func (m MemtableEntry) HashKey() string {
	return string(m.Key)
}

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

type MemtableType string

const (
	TypeBTree     MemtableType = "btree"
	TypeRBTree    MemtableType = "rbtree"
	TypeSkipList  MemtableType = "skiplist"
	TypeHashMap   MemtableType = "hashmap"
	TypeAVLTree   MemtableType = "avltree"
	TypeHashMapSL MemtableType = "hashmapsl"
)

type GenericMemtable struct {
	store         MemtableStore
	numEntries    int
	sizeBytes     uint64
	maxNumEntries int
	maxSizeBytes  uint64

	flushHandler func([]MemtableEntry)
}
