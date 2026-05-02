package memtable

import (
	"bytes"
	"math"
	"sort"
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
	wg             sync.WaitGroup        // wait for flush
	closing        bool
	snapshots      map[string]struct{}
}

// NewMemtableManager initializes a new instance of a manager and starts the flush worker
func NewMemtableManager(maxTables int, mergeStructure byte, factory func() Memtable, flushHandler func([]MemtableEntry, []MemtableEntry)) *MemtableManager {
	mm := &MemtableManager{
		maxTables:      maxTables,
		mergeStructure: mergeStructure,
		factory:        factory,
		flushChannel:   make(chan Memtable, maxTables),
		snapshots:      make(map[string]struct{}),
	}
	mm.cond = sync.NewCond(&mm.mu)
	mm.active = mm.factory()
	go mm.flushWorker(flushHandler)
	return mm
}

// Snapshot marks key so that all future writes to it create new versions
// rather than overwriting the previous one.
func (mm *MemtableManager) Snapshot(key []byte) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.snapshots[string(key)] = struct{}{}
}

func (mm *MemtableManager) isSnapshotted(key []byte) bool {
	_, ok := mm.snapshots[string(key)]
	return ok
}

// Put inserts an entry into active memtable and if the memtable reaches max capacity, triggers rotation
func (mm *MemtableManager) Put(key []byte, value []byte, seqId uint64, opType enums.OpType) {
	mm.mu.Lock()
	if opType == enums.OpTypeRangeDel || mm.isSnapshotted(key) {
		mm.active.Put(key, value, seqId, opType)
	} else {
		mm.active.Upsert(key, value, seqId, opType)
	}
	shouldRotate := mm.active.ShouldFlush()
	mm.mu.Unlock()
	if shouldRotate {
		mm.rotate()
	}
}

func (mm *MemtableManager) PutWithTTL(key []byte, value []byte, seqId uint64, opType enums.OpType, ttl int64) {
	mm.mu.Lock()
	if mm.isSnapshotted(key) {
		mm.active.PutWithTTL(key, value, seqId, opType, ttl)
	} else {
		mm.active.UpsertWithTTL(key, value, seqId, opType, ttl)
	}
	shouldRotate := mm.active.ShouldFlush()
	mm.mu.Unlock()
	if shouldRotate {
		mm.rotate()
	}
}

// Remove physically deletes the entry with the given key+seqId from the active memtable.
func (mm *MemtableManager) Remove(key []byte, seqId uint64) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.active.Remove(key, seqId)
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
	mm.wg.Add(1)
	mm.mu.Unlock()
	mm.flushChannel <- immutable
}

// flushWorker runs in a separate goroutine, and it takes immutable memtables and flushes them to disk
func (mm *MemtableManager) flushWorker(flushHandler func([]MemtableEntry, []MemtableEntry)) {
	for mem := range mm.flushChannel {
		mm.mu.Lock()
		shouldFlush := mm.containsImmutable(mem)
		mm.removeImmutable(mem)
		mm.cond.Signal()
		mm.mu.Unlock()

		if shouldFlush {
			entries, rangeDelEntries := mem.Flush()
			if flushHandler != nil {
				flushHandler(entries, rangeDelEntries)
			}
		}

		mm.wg.Done()
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
		activeIt := NewRawSingleMemtableIterator(mm.active.RawIterator())
		rawIters = append(rawIters, activeIt.(*RawSingleMemtableIterator))
	}
	for i := len(mm.immutable) - 1; i >= 0; i-- {
		it := NewRawSingleMemtableIterator(mm.immutable[i].RawIterator())
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
	checker := func(key []byte, seqId uint64) bool {
		if mm.active.IsCoveredByRangeDel(key, seqId) {
			return true
		}
		for i := len(mm.immutable) - 1; i >= 0; i-- {
			if mm.immutable[i].IsCoveredByRangeDel(key, seqId) {
				return true
			}
		}
		return false
	}
	return NewMergedMemtableIterator(rawMerge, checker)
}

func (mm *MemtableManager) ResetAll() {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	mm.active = mm.factory()
	mm.immutable = nil
}

func (mm *MemtableManager) Close() {
	mm.mu.Lock()
	if mm.active != nil && mm.active.ShouldFlush() {
		immutable := mm.active
		mm.immutable = append(mm.immutable, immutable)
		mm.active = mm.factory()
		mm.wg.Add(1)
		mm.mu.Unlock()
		mm.flushChannel <- immutable
	} else {
		mm.mu.Unlock()
	}

	close(mm.flushChannel)
	mm.wg.Wait()
}

// EntryIterator returns a MergedMemtableIterator adapted to iterator.Entry type
// This is used by DBIterator which works at the Entry level
func (mm *MemtableManager) EntryIterator() iterator.Iterator[iterator.Entry] {
	typedIt := mm.RawIterator() // using raw in order to DBIterator see tombstone
	return iterator.NewAdaptedIterator(
		&memtableIteratorSeekWrapper{inner: typedIt},
		func(e MemtableEntry) iterator.Entry {
			return iterator.Entry{
				Key:        append([]byte(nil), e.Key...),
				Value:      append([]byte(nil), e.Value...),
				OpType:     e.OpType,
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

func (mm *MemtableManager) IsCoveredByRangeDel(key []byte, keySeqId uint64) bool {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	if mm.active.IsCoveredByRangeDel(key, keySeqId) {
		return true
	}
	for i := len(mm.immutable) - 1; i >= 0; i-- {
		if mm.immutable[i].IsCoveredByRangeDel(key, keySeqId) {
			return true
		}
	}
	return false
}

// GetVersions returns all stored versions of key from all memtable instances, newest first.
func (mm *MemtableManager) GetVersions(key []byte, maxVersions int) []MemtableEntry {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	seen := make(map[uint64]struct{})
	var versions []MemtableEntry

	collect := func(mem Memtable) {
		it := mem.RawIterator()
		target := MemtableEntry{Key: key, SeqId: math.MaxUint64}
		it.Seek(target)
		for it.Valid() {
			entry := it.Key()
			if !bytes.Equal(entry.Key, key) {
				break
			}
			if entry.OpType != enums.OpTypeDel {
				if _, dup := seen[entry.SeqId]; !dup {
					seen[entry.SeqId] = struct{}{}
					versions = append(versions, entry)
				}
			}
			it.Next()
			if maxVersions > 0 && len(versions) >= maxVersions {
				return
			}
		}
	}

	collect(mm.active)
	for i := len(mm.immutable) - 1; i >= 0; i-- {
		collect(mm.immutable[i])
		if maxVersions > 0 && len(versions) >= maxVersions {
			break
		}
	}

	sort.Slice(versions, func(i, j int) bool {
		return versions[i].SeqId > versions[j].SeqId
	})
	return versions
}
