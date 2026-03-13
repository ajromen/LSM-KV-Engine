package memtable

import (
	"sync"

	"github.com/ajromen/LSM-KV-Engine/internal/include"
)

type MemtableManager struct {
	active         Memtable
	immutable      []Memtable
	maxTables      int
	mergeStructure byte
	factory        func() Memtable
	flushChannel   chan Memtable
	flushHandler   func([]MemtableEntry)
	mu             sync.Mutex
	cond           *sync.Cond
}

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

func (mm *MemtableManager) Put(key []byte, value []byte, timestamp uint64, tombstone bool) {
	mm.mu.Lock()
	mm.active.Put(key, value, timestamp, tombstone)
	shouldRotate := mm.active.ShouldFlush()
	mm.mu.Unlock()
	if shouldRotate {
		mm.rotate()
	}
}

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

func (mm *MemtableManager) flushWorker(flushHandler func([]MemtableEntry)) {
	for mem := range mm.flushChannel {
		mm.mu.Lock()
		mm.removeImmutable(mem)
		mm.cond.Signal()
		mm.mu.Unlock()
		entries := mem.Flush()
		if flushHandler != nil {
			flushHandler(entries)
		}
	}
}

func (mm *MemtableManager) removeImmutable(target Memtable) {
	newList := make([]Memtable, 0, len(mm.immutable)-1)
	for _, m := range mm.immutable {
		if m != target {
			newList = append(newList, m)
		}
	}
	mm.immutable = newList
}

func (mm *MemtableManager) Get(key []byte) ([]byte, bool) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	if v, ok := mm.active.Get(key); ok {
		return v, true
	}
	for i := len(mm.immutable) - 1; i >= 0; i-- {
		if v, ok := mm.immutable[i].Get(key); ok {
			return v, true
		}
	}
	return nil, false
}

func (mm *MemtableManager) Delete(key []byte, timestamp uint64) {
	mm.mu.Lock()
	mm.active.Put(key, []byte{}, timestamp, true)
	shouldRotate := mm.active.ShouldFlush()
	mm.mu.Unlock()
	if shouldRotate {
		mm.rotate()
	}
}

func (mm *MemtableManager) RawIterator() include.Iterator[MemtableEntry] {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	var rawIters []include.Iterator[MemtableEntry]
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

func (mm *MemtableManager) Iterator() include.Iterator[MemtableEntry] {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	var rawIters []include.Iterator[MemtableEntry]
	if mm.active != nil {
		rawIters = append(rawIters, NewRawSingleMemtableIterator(mm.active.Iterator()))
	}
	for i := len(mm.immutable) - 1; i >= 0; i-- {
		rawIters = append(rawIters, NewRawSingleMemtableIterator(mm.immutable[i].Iterator()))
	}

	rawMerge := NewRawIterator(rawIters, mm.mergeStructure)
	return NewMergedMemtableIterator(rawMerge)
}
