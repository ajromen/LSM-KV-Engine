package memtable

import (
	"sync"

	"github.com/ajromen/LSM-KV-Engine/internal/data_structures"
	"github.com/ajromen/LSM-KV-Engine/internal/include"
)

type MemtableFactory struct {
	active       Memtable
	immutable    []Memtable
	maxTables    int
	factory      func() Memtable
	flushChannel chan Memtable
	flushHandler func([]MemtableEntry)
	mu           sync.Mutex
	cond         *sync.Cond
}

func NewMemtableFactory(maxTables int, factory func() Memtable, flushHandler func([]MemtableEntry)) *MemtableFactory {
	f := &MemtableFactory{
		maxTables:    maxTables,
		factory:      factory,
		flushChannel: make(chan Memtable, maxTables),
	}
	f.cond = sync.NewCond(&f.mu)
	f.active = f.factory()
	go f.flushWorker(flushHandler)
	return f
}

func (f *MemtableFactory) Put(key []byte, value []byte, timestamp uint64, tombstone bool) {
	f.mu.Lock()
	f.active.Put(key, value, timestamp, tombstone)
	shouldRotate := f.active.ShouldFlush()
	f.mu.Unlock()
	if shouldRotate {
		f.rotate()
	}
}

func (f *MemtableFactory) rotate() {
	f.mu.Lock()
	if !f.active.ShouldFlush() {
		f.mu.Unlock()
		return
	}
	for len(f.immutable) >= f.maxTables {
		f.cond.Wait()
	}
	immutable := f.active
	f.immutable = append(f.immutable, immutable)
	f.active = f.factory()
	f.mu.Unlock()
	f.flushChannel <- immutable
}

func (f *MemtableFactory) flushWorker(flushHandler func([]MemtableEntry)) {
	for mem := range f.flushChannel {
		f.mu.Lock()
		f.removeImmutable(mem)
		f.cond.Signal()
		f.mu.Unlock()
		entries := mem.Flush()
		if flushHandler != nil {
			flushHandler(entries)
		}
	}
}

func (f *MemtableFactory) removeImmutable(target Memtable) {
	newList := make([]Memtable, 0, len(f.immutable)-1)
	for _, m := range f.immutable {
		if m != target {
			newList = append(newList, m)
		}
	}
	f.immutable = newList
}

func (f *MemtableFactory) Get(key []byte) ([]byte, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if v, ok := f.active.Get(key); ok {
		return v, true
	}
	for i := len(f.immutable) - 1; i >= 0; i-- {
		if v, ok := f.immutable[i].Get(key); ok {
			return v, true
		}
	}
	return nil, false
}

func (f *MemtableFactory) Delete(key []byte, timestamp uint64) {
	f.mu.Lock()
	f.active.Put(key, []byte{}, timestamp, true)
	shouldRotate := f.active.ShouldFlush()
	f.mu.Unlock()
	if shouldRotate {
		f.rotate()
	}
}

func (f *MemtableFactory) RawIterator() include.Iterator[MemtableEntry] {
	f.mu.Lock()
	defer f.mu.Unlock()
	var rawIters []include.Iterator[MemtableEntry]
	if f.active != nil {
		activeIt := NewRawSingleMemtableIterator(f.active.Iterator())
		rawIters = append(rawIters, activeIt.(*RawSingleMemtableIterator))
	}
	for i := len(f.immutable) - 1; i >= 0; i-- {
		it := NewRawSingleMemtableIterator(f.immutable[i].Iterator())
		rawIters = append(rawIters, it.(*RawSingleMemtableIterator))
	}
	return NewRawIterator(rawIters, data_structures.Heap)
}

func (f *MemtableFactory) Iterator() include.Iterator[MemtableEntry] {
	f.mu.Lock()
	defer f.mu.Unlock()

	var rawIters []include.Iterator[MemtableEntry]
	if f.active != nil {
		rawIters = append(rawIters, NewRawSingleMemtableIterator(f.active.Iterator()))
	}
	for i := len(f.immutable) - 1; i >= 0; i-- {
		rawIters = append(rawIters, NewRawSingleMemtableIterator(f.immutable[i].Iterator()))
	}

	rawMerge := NewRawIterator(rawIters, data_structures.Heap)
	return NewMergedMemtableIterator(rawMerge)
}
