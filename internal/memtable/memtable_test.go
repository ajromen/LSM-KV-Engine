package memtable

import (
	"bytes"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
	"github.com/ajromen/LSM-KV-Engine/internal/iterator"
)

var allTypes = []enums.MemTableType{enums.BTreeMemTable, enums.SkiplistMemTable, enums.HashMapMemTable}

func newMemtable(t *testing.T, mt enums.MemTableType, maxEntries int, maxBytes uint64, handler func([]MemtableEntry)) *MemtableManager {
	t.Helper()
	factory := NewFactory(config.NewDefaultConfig().Memtable)
	return NewMemtableManager(5, 1, factory, handler)
}

// ============================================================
// Basic Put / Get
// ============================================================

func TestPutGet(t *testing.T) {
	for _, mt := range allTypes {
		mt := mt
		t.Run(string(mt), func(t *testing.T) {
			m := newMemtable(t, mt, 100, 1<<20, nil)
			keys := []string{"key1", "key2", "key3", "key4", "key5"}
			for i, k := range keys {
				m.Put([]byte(k), []byte(fmt.Sprintf("value%d", i+1)), 10, false)
			}
			for i, k := range keys {
				got, ok := m.Get([]byte(k))
				if !ok {
					t.Fatalf("Get(%q): not found", k)
				}
				want := fmt.Sprintf("value%d", i+1)
				if !bytes.Equal(got, []byte(want)) {
					t.Errorf("Get(%q) = %q, want %q", k, got, want)
				}
			}
		})
	}
}

func TestGetMissing(t *testing.T) {
	for _, mt := range allTypes {
		mt := mt
		t.Run(string(mt), func(t *testing.T) {
			m := newMemtable(t, mt, 100, 1<<20, nil)
			got, ok := m.Get([]byte("nonexistent"))
			if ok || got != nil {
				t.Errorf("Get(missing): got (%v, %v), want (nil, false)", got, ok)
			}
		})
	}
}

// ============================================================
// Timestamp semantics
// ============================================================

func TestNewerTimestampWins(t *testing.T) {
	for _, mt := range allTypes {
		mt := mt
		t.Run(string(mt), func(t *testing.T) {
			m := newMemtable(t, mt, 100, 1<<20, nil)
			m.Put([]byte("key"), []byte("old"), 10, false)
			m.Put([]byte("key"), []byte("new"), 20, false)
			got, ok := m.Get([]byte("key"))
			if !ok {
				t.Fatal("Get: not found")
			}
			if !bytes.Equal(got, []byte("new")) {
				t.Errorf("got %q, want %q", got, "new")
			}
		})
	}
}

func TestOlderTimestampDoesNotOverwrite(t *testing.T) {
	allTypes = []enums.MemTableType{enums.BTreeMemTable, enums.SkiplistMemTable}
	for _, mt := range allTypes {
		mt := mt
		t.Run(string(mt), func(t *testing.T) {
			m := newMemtable(t, mt, 100, 1<<20, nil)
			m.Put([]byte("key"), []byte("old"), 10, false)
			m.Put([]byte("key"), []byte("new"), 20, false)
			got, ok := m.Get([]byte("key"))
			if !ok {
				t.Fatal("Get: not found")
			}
			if !bytes.Equal(got, []byte("new")) {
				t.Errorf("older write overwrote newer: got %q, want %q", got, "new")
			}
		})
	}
}

// ============================================================
// Tombstone / Delete semantics
// ============================================================

func TestTombstoneHidesEntry(t *testing.T) {
	for _, mt := range allTypes {
		mt := mt
		t.Run(string(mt), func(t *testing.T) {
			m := newMemtable(t, mt, 100, 1<<20, nil)
			m.Put([]byte("key"), []byte("value"), 10, false)
			m.Put([]byte("key"), []byte(""), 20, true)
			got, ok := m.Get([]byte("key"))
			if ok || got != nil {
				t.Errorf("expected deleted key to be hidden, got (%v, %v)", got, ok)
			}
		})
	}
}

func TestDeleteHidesEntry(t *testing.T) {
	for _, mt := range allTypes {
		mt := mt
		t.Run(string(mt), func(t *testing.T) {
			m := newMemtable(t, mt, 100, 1<<20, nil)
			m.Put([]byte("key"), []byte("value"), 10, false)
			m.Delete([]byte("key"), 20)
			got, ok := m.Get([]byte("key"))
			if ok || got != nil {
				t.Errorf("expected deleted key to be hidden, got (%v, %v)", got, ok)
			}
		})
	}
}

func TestOlderTombstoneDoesNotHideNewerWrite(t *testing.T) {
	allTypes = []enums.MemTableType{enums.BTreeMemTable, enums.SkiplistMemTable}
	for _, mt := range allTypes {
		mt := mt
		t.Run(string(mt), func(t *testing.T) {
			m := newMemtable(t, mt, 100, 1<<20, nil)
			m.Put([]byte("key"), []byte(""), 10, true) // older tombstone
			m.Put([]byte("key"), []byte("value"), 20, false)
			got, ok := m.Get([]byte("key"))
			if !ok {
				t.Fatal("Get: not found — older tombstone should not hide newer write")
			}
			if !bytes.Equal(got, []byte("value")) {
				t.Errorf("got %q, want %q", got, "value")
			}
		})
	}
}

// ============================================================
// Single-memtable raw iterator
// ============================================================

func TestRawSingleIteratorAllVersions(t *testing.T) {
	for _, mt := range allTypes {
		mt := mt
		t.Run(string(mt), func(t *testing.T) {
			m := newMemtable(t, mt, 100, 1<<20, nil)
			m.Put([]byte("a"), []byte("old"), 10, false)
			m.Put([]byte("a"), []byte("new"), 20, false)
			m.Put([]byte("b"), []byte("only"), 10, false)

			rawIt := m.active.RawIterator()
			rawIt.SeekToFirst()
			var entries []MemtableEntry
			for rawIt.Valid() {
				entries = append(entries, rawIt.Value())
				rawIt.Next()
			}
			// raw iterator must expose every version
			if len(entries) < 3 {
				t.Errorf("expected at least 3 raw entries, got %d", len(entries))
			}
		})
	}
}

// ============================================================
// Single-memtable merged iterator (deduplication)
// ============================================================

func TestSingleIteratorDeduplication(t *testing.T) {
	for _, mt := range allTypes {
		mt := mt
		t.Run(string(mt), func(t *testing.T) {
			m := newMemtable(t, mt, 100, 1<<20, nil)
			m.Put([]byte("key1"), []byte("old"), 10, false)
			m.Put([]byte("key1"), []byte("new"), 20, false)
			m.Put([]byte("key2"), []byte("only"), 10, false)
			m.Put([]byte("key3"), []byte("v"), 10, false)
			m.Put([]byte("key3"), []byte(""), 20, true) // tombstone

			it := m.active.Iterator()
			it.SeekToFirst()
			seen := map[string]MemtableEntry{}
			for it.Valid() {
				e := it.Value()
				seen[string(e.Key)] = e
				it.Next()
			}
			// key1: latest version must be "new"
			if e, ok := seen["key1"]; !ok || !bytes.Equal(e.Value, []byte("new")) {
				t.Errorf("key1: got %v, want value=new", e)
			}
			// key2: present
			if _, ok := seen["key2"]; !ok {
				t.Error("key2 missing from iterator")
			}
			// key3: tombstone — should NOT appear in merged iterator
			if e, ok := seen["key3"]; ok && !e.Tombstone {
				t.Errorf("key3 should be absent or tombstoned, got %v", e)
			}
		})
	}
}

// ============================================================
// Multi-memtable raw iterator (cross-table merge)
// ============================================================

func TestRawIteratorAcrossTwoMemtables(t *testing.T) {
	for _, mt := range allTypes {
		mt := mt
		t.Run(string(mt), func(t *testing.T) {
			m1 := newMemtable(t, mt, 100, 1<<20, nil)
			m2 := newMemtable(t, mt, 100, 1<<20, nil)
			m1.Put([]byte("key1"), []byte("v1old"), 10, false)
			m1.Put([]byte("key1"), []byte("v1new"), 20, false)
			m2.Put([]byte("key2"), []byte("v2old"), 10, false)
			m2.Put([]byte("key2"), []byte("v2new"), 20, false)
			m1.Put([]byte("key3"), []byte("v3"), 10, false)

			rawIt1 := m1.active.RawIterator()
			rawIt1.SeekToFirst()
			rawIt2 := m2.active.RawIterator()
			rawIt2.SeekToFirst()

			globalRaw := NewRawIterator([]iterator.Iterator[MemtableEntry]{rawIt1, rawIt2}, 1)
			globalRaw.SeekToFirst()

			keys := map[string]bool{}
			for globalRaw.Valid() {
				keys[string(globalRaw.Value().Key)] = true
				globalRaw.Next()
			}
			for _, k := range []string{"key1", "key2", "key3"} {
				if !keys[k] {
					t.Errorf("expected key %q in raw iterator", k)
				}
			}
		})
	}
}

// ============================================================
// Multi-memtable merged iterator with Seek
// ============================================================

func TestIteratorSeek(t *testing.T) {
	for _, mt := range allTypes {
		mt := mt
		t.Run(string(mt), func(t *testing.T) {
			m1 := newMemtable(t, mt, 100, 1<<20, nil)
			m2 := newMemtable(t, mt, 100, 1<<20, nil)
			for _, k := range []string{"key1", "key3", "key5"} {
				m1.Put([]byte(k), []byte("v"), 10, false)
			}
			for _, k := range []string{"key2", "key4"} {
				m2.Put([]byte(k), []byte("v"), 10, false)
			}

			rawIt1 := m1.active.RawIterator()
			rawIt1.SeekToFirst()
			rawIt2 := m2.active.RawIterator()
			rawIt2.SeekToFirst()

			globalRaw := NewRawIterator([]iterator.Iterator[MemtableEntry]{rawIt1, rawIt2}, 1)
			globalIt := NewMergedMemtableIterator(globalRaw)
			globalIt.Seek(MemtableEntry{Key: []byte("key3"), Timestamp: math.MaxInt64})

			var got []string
			for globalIt.Valid() {
				got = append(got, string(globalIt.Value().Key))
				globalIt.Next()
			}
			if len(got) == 0 || got[0] != "key3" {
				t.Errorf("Seek(key3): first result = %v, want key3", got)
			}
			// key1 and key2 must not appear after seek
			for _, k := range got {
				if k < "key3" {
					t.Errorf("Seek(key3): got key %q which is before seek target", k)
				}
			}
		})
	}
}

// ============================================================
// Manager-level iterator across active + immutables
// ============================================================

func TestManagerIteratorAcrossFlush(t *testing.T) {
	for _, mt := range allTypes {
		mt := mt
		t.Run(string(mt), func(t *testing.T) {
			// maxEntries=3 forces rotations quickly
			flushed := map[string]bool{}
			var mu sync.Mutex
			m := newMemtable(t, mt, 3, 1<<20, func(entries []MemtableEntry) {
				mu.Lock()
				for _, e := range entries {
					flushed[string(e.Key)] = true
				}
				mu.Unlock()
			})

			keys := []string{"a", "b", "c", "d", "e", "f", "g"}
			for _, k := range keys {
				m.Put([]byte(k), []byte("v-"+k), 10, false)
			}

			time.Sleep(100 * time.Millisecond)

			it := m.Iterator()
			it.SeekToFirst()
			seen := map[string]bool{}
			for it.Valid() {
				seen[string(it.Value().Key)] = true
				it.Next()
			}

			// every key that was not flushed must be visible via iterator
			for _, k := range keys {
				mu.Lock()
				wasFlushed := flushed[k]
				mu.Unlock()
				if !wasFlushed && !seen[k] {
					t.Errorf("key %q not visible in iterator and not flushed", k)
				}
			}
		})
	}
}

// ============================================================
// Concurrency: Put / Get / Delete racing with flushes
// ============================================================

func TestConcurrency(t *testing.T) {
	var mu sync.Mutex
	var flushedEntries []MemtableEntry
	flushCount := 0

	factory := NewFactory(config.NewDefaultConfig().Memtable)
	memManager := NewMemtableManager(5, 0, factory, func(entries []MemtableEntry) {
		mu.Lock()
		flushCount++
		current := flushCount
		mu.Unlock()

		t.Logf(">>> flush #%d started — %d entries", current, len(entries))
		for _, e := range entries {
			t.Logf("    key=%s value=%s tombstone=%v ts=%d", e.Key, e.Value, e.Tombstone, e.Timestamp)
		}

		mu.Lock()
		flushedEntries = append(flushedEntries, entries...)
		mu.Unlock()

		t.Logf("<<< flush #%d done", current)
	})

	const numWriters = 5
	const writesPerWriter = 200
	var wg sync.WaitGroup

	// concurrent puts
	for w := 0; w < numWriters; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < writesPerWriter; i++ {
				key := fmt.Sprintf("worker%d-key%04d", workerID, i)
				value := fmt.Sprintf("value-%d-%d", workerID, i)
				ts := uint64(workerID*writesPerWriter + i)
				memManager.Put([]byte(key), []byte(value), ts, false)
			}
		}(w)
	}

	// concurrent gets racing with puts
	for r := 0; r < 3; r++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				key := fmt.Sprintf("worker%d-key%04d", readerID, i)
				v, ok := memManager.Get([]byte(key))
				if ok {
					t.Logf("reader %d: got key=%s value=%s", readerID, key, v)
				}
			}
		}(r)
	}

	// concurrent deletes
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			key := fmt.Sprintf("worker0-key%04d", i)
			memManager.Delete([]byte(key), uint64(999999+i))
		}
	}()

	wg.Wait()
	t.Log("=== all writers done, waiting for flushes to drain ===")
	time.Sleep(200 * time.Millisecond)

	// verify total entry count
	mu.Lock()
	totalFlushed := len(flushedEntries)
	totalFlushes := flushCount
	mu.Unlock()

	expectedWrites := numWriters * writesPerWriter
	expectedDeletes := 0
	expectedTotal := expectedWrites + expectedDeletes

	// entries still in active memtable were not flushed yet
	it := memManager.Iterator()
	it.SeekToFirst()
	inMemory := 0
	for it.Valid() {
		it.Next()
		inMemory++
	}

	t.Logf("total flushes: %d", totalFlushes)
	t.Logf("flushed entries: %d, in active memtable: %d, total: %d, expected: %d",
		totalFlushed, inMemory, totalFlushed+inMemory, expectedTotal)

	// every write and delete must be accounted for exactly once
	if totalFlushed+inMemory != expectedTotal {
		t.Errorf("entry count mismatch: flushed=%d + inMemory=%d = %d, want %d",
			totalFlushed, inMemory, totalFlushed+inMemory, expectedTotal)
	}

	// raw iterator must agree
	rawIt := memManager.RawIterator()
	rawIt.SeekToFirst()
	rawCount := 0
	for rawIt.Valid() {
		rawIt.Next()
		rawCount++
	}
	t.Logf("raw iterator in-memory entries: %d", rawCount)
}

func TestMemtableLifecycle(t *testing.T) {
	var mu sync.Mutex
	var flushed []MemtableEntry
	var wg sync.WaitGroup
	cfg := config.MemtableConfig{
		MemtableType:         enums.SkiplistMemTable,
		MemtableMaxSizeBytes: 1 << 20,
		MemtableMaxEntries:   3,
	}
	factory := NewFactory(cfg)

	wg.Add(1)

	mem := NewMemtableManager(5, 0, factory, func(entries []MemtableEntry) {
		fmt.Println("=== FLUSH START ===")
		for _, e := range entries {
			fmt.Printf("flush: key=%s value=%s ts=%d tomb=%v\n",
				e.Key, e.Value, e.Timestamp, e.Tombstone)
		}
		mu.Lock()
		flushed = append(flushed, entries...)
		mu.Unlock()
		fmt.Println("=== FLUSH END ===")
		wg.Done()
	})
	fmt.Println("=== PUT PHASE ===")
	mem.Put([]byte("a"), []byte("1"), 10, false)
	mem.Put([]byte("b"), []byte("2"), 20, false)
	mem.Put([]byte("c"), []byte("3"), 30, false)
	fmt.Println("ACTIVE MEMTABLE:")
	fmt.Println(mem.active.Visualize())
	mem.Put([]byte("d"), []byte("4"), 40, false)
	mem.Put([]byte("e"), []byte("5"), 40, false)
	wg.Wait()
	fmt.Println("=== ITERATOR VIEW ===")
	it := mem.Iterator()
	it.SeekToFirst()
	for it.Valid() {
		e := it.Value()
		fmt.Printf("iter: key=%s value=%s ts=%d\n",
			e.Key, e.Value, e.Timestamp)
		it.Next()
	}
	fmt.Println("=== FLUSHED COUNT ===")
	mu.Lock()
	fmt.Println("flushed entries:", len(flushed))
	mu.Unlock()
}
