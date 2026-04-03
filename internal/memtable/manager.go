package memtable

import (
	"sync"
	"time"

	"github.com/ajromen/LSM-KV-Engine/internal/enums"
	"github.com/ajromen/LSM-KV-Engine/internal/iterator"
)

// MemtableManager coordinates multiple memtable instances in a whole lsm engine
// It maintains one active memtable and multiple immutable memtables waiting to be flushed in background to disk
type MemtableManager struct {
	active         Memtable              // current writeable memtable instance
	immutable      []Memtable            // list of memtables that are being flushed in background
	maxTables      int                   // max number of immutable memtables allowed
	mergeStructure byte                  // strategy for merging iterators of active+n instances of immutable memtables
	factory        func() Memtable       // factory for creating new memtables
	flushChannel   chan Memtable         // channel for async flushing
	flushHandler   func([]MemtableEntry) // function that persists flushed entries
	mu             sync.Mutex            // protets all shared states
	cond           *sync.Cond            // used to block when to many immutable instances
}

// NewMemtableManager initializes a new instance of a manager and starts the flush worker
func NewMemtableManager(maxTables int, mergeStructure byte, factory func() Memtable, flushHandler func([]MemtableEntry)) *MemtableManager {
	mm := &MemtableManager{
		maxTables:      maxTables,
		mergeStructure: mergeStructure,
		factory:        factory,
		flushChannel:   make(chan Memtable, maxTables),
	}
	mm.cond = sync.NewCond(&mm.mu)
	mm.active = mm.factory()
	go mm.flushWorker(flushHandler)
	return mm
}

// Put inserts an entry into active memtable and if the memtable reaches max capacity, triggers rotation
func (mm *MemtableManager) Put(key []byte, value []byte, seqId uint64, opType enums.OpType) {
	mm.mu.Lock()
	mm.active.Put(key, value, seqId, opType)
	shouldRotate := mm.active.ShouldFlush()
	mm.mu.Unlock()
	if shouldRotate {
		mm.rotate()
	}
}

func (mm *MemtableManager) PutWithTTL(key []byte, value []byte, seqId uint64, opType enums.OpType, ttl int64) {
	mm.mu.Lock()
	mm.active.PutWithTTL(key, value, seqId, opType, ttl)
	shouldRotate := mm.active.ShouldFlush()
	mm.mu.Unlock()
	if shouldRotate {
		mm.rotate()
	}
}

// rotate moves the active memtable to immutable list and creates a new one
// if too many immutable instances of memtable exist, it blocks new puts until space is available
func (mm *MemtableManager) rotate() {
	mm.mu.Lock()
	if !mm.active.ShouldFlush() {
		mm.mu.Unlock()
		return
	}
	for len(mm.immutable) >= mm.maxTables {
		mm.cond.Wait()
	}
	immutable := mm.active
	mm.immutable = append(mm.immutable, immutable)
	mm.active = mm.factory()
	mm.mu.Unlock()
	mm.flushChannel <- immutable
}

// flushWorker runs in a separate goroutine, and it takes immutable memtables and flushes them to disk
func (mm *MemtableManager) flushWorker(flushHandler func([]MemtableEntry)) {
	for mem := range mm.flushChannel {
		mm.mu.Lock()
		shouldFlush := mm.containsImmutable(mem)
		mm.removeImmutable(mem)
		mm.cond.Signal()
		mm.mu.Unlock()

		if !shouldFlush {
			continue
		}

		entries := mem.Flush()
		if flushHandler != nil {
			flushHandler(entries)
		}
	}
}

// removeImmutable removes a specific memtable from the immutable slice
func (mm *MemtableManager) removeImmutable(target Memtable) {
	newList := make([]Memtable, 0, len(mm.immutable)-1)
	for _, m := range mm.immutable {
		if m != target {
			newList = append(newList, m)
		}
	}
	mm.immutable = newList
}

func (mm *MemtableManager) containsImmutable(target Memtable) bool {
	for _, m := range mm.immutable {
		if m == target {
			return true
		}
	}
	return false
}

func (mm *MemtableManager) Get(key []byte) (*MemtableEntry, bool) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	if v, ok := mm.active.Get(key); ok {
		return mm.checkTTL(v, ok)
	}
	for i := len(mm.immutable) - 1; i >= 0; i-- {
		if v, ok := mm.immutable[i].Get(key); ok {
			return mm.checkTTL(v, ok)
		}
	}
	return nil, false
}

func (mm *MemtableManager) checkTTL(entry *MemtableEntry, ok bool) (*MemtableEntry, bool) {
	if entry == nil {
		return nil, true
	}
	if entry.ExpiresAt != 0 && time.UnixMilli(entry.ExpiresAt).Before(time.Now()) {
		return nil, false
	}
	return entry, ok
}

// RawIterator returns a merged iterator over all memtables without higher-level filtering.
func (mm *MemtableManager) RawIterator() iterator.Iterator[MemtableEntry] {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	var rawIters []iterator.Iterator[MemtableEntry]
	if mm.active != nil {
		activeIt := NewRawSingleMemtableIterator(mm.active.Iterator())
		rawIters = append(rawIters, activeIt.(*RawSingleMemtableIterator))
	}
	for i := len(mm.immutable) - 1; i >= 0; i-- {
		it := NewRawSingleMemtableIterator(mm.immutable[i].Iterator())
		rawIters = append(rawIters, it.(*RawSingleMemtableIterator))
	}
	return NewRawIterator(rawIters, mm.mergeStructure)
}

// Iterator returns a merged iterator over all memtables with higher-level filtering.
func (mm *MemtableManager) Iterator() iterator.Iterator[MemtableEntry] {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	var rawIters []iterator.Iterator[MemtableEntry]
	if mm.active != nil {
		rawIters = append(rawIters, NewRawSingleMemtableIterator(mm.active.Iterator()))
	}
	for i := len(mm.immutable) - 1; i >= 0; i-- {
		rawIters = append(rawIters, NewRawSingleMemtableIterator(mm.immutable[i].Iterator()))
	}

	rawMerge := NewRawIterator(rawIters, mm.mergeStructure)
	return NewMergedMemtableIterator(rawMerge)
}

func (mm *MemtableManager) ResetAll() {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	mm.active = mm.factory()
	mm.immutable = nil
}

func (mm *MemtableManager) Close() {
	//close(mm.flushChannel) mozda treba
}

// EntryIterator returns a MergedMemtableIterator adapted to iterator.Entry type
// This is used by DBIterator which works at the Entry level
func (mm *MemtableManager) EntryIterator() iterator.Iterator[iterator.Entry] {
	typedIt := mm.Iterator() // Iterator[MemtableEntry]
	return iterator.NewAdaptedIterator(
		&memtableIteratorSeekWrapper{inner: typedIt},
		func(e MemtableEntry) iterator.Entry {
			return iterator.Entry{
				Key:        append([]byte(nil), e.Key...),
				Value:      append([]byte(nil), e.Value...),
				Tombstone:  e.OpType == enums.OpTypeDel,
				SequenceID: e.SeqId,
			}
		},
		func(key []byte) MemtableEntry {
			return MemtableEntry{Key: key}
		},
	)
}

// memtableIteratorSeekWrapper wraps Iterator[MemtableEntry] to add TypedSeekIterator interface
type memtableIteratorSeekWrapper struct {
	inner iterator.Iterator[MemtableEntry]
}

func (w *memtableIteratorSeekWrapper) Valid() bool          { return w.inner.Valid() }
func (w *memtableIteratorSeekWrapper) SeekToFirst()         { w.inner.SeekToFirst() }
func (w *memtableIteratorSeekWrapper) SeekToLast()          { w.inner.SeekToLast() }
func (w *memtableIteratorSeekWrapper) Next()                { w.inner.Next() }
func (w *memtableIteratorSeekWrapper) Key() MemtableEntry   { return w.inner.Key() }
func (w *memtableIteratorSeekWrapper) Seek(e MemtableEntry) { w.inner.Seek(e) }
