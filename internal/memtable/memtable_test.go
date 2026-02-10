package memtable

import (
	"testing"
)

func TestHashmap(t *testing.T) {
	t.Log("---- HASHMAP MEMTABLE TEST ----")
	mt := NewMemtables("hashmap", 1, 10, func(entries []MemtableEntry) {})
	mt.Put("key1", []byte("value1"))
	mt.Put("key2", []byte("value2"))
	mt.Put("key3", []byte("value3"))
	mt.Put("key4", []byte("value4"))
	entry, ok := mt.Get("key1")
	if !ok {
		t.Fatal("key1 not found, expected to be found")
	}
	if string(entry.Value) != "value1" {
		t.Fatalf("expected value1, got %s", entry.Value)
	}
	mt.Put("key1", []byte("value1_updated"))
	entryUpdated, ok := mt.Get("key1")
	if !ok {
		t.Fatalf("key1 not found")
	}
	if string(entryUpdated.Value) != "value1_updated" {
		t.Fatalf("expected value1_updated, got %s", entryUpdated.Value)
	}
	_, ok = mt.Get("nonexisting")
	if ok {
		t.Fatalf("expected nonexisting key to be missing")
	}
	mt.Delete("nonexisting")
	entries := mt.ReadEntriesNoFlushing()
	if !entries[4].Tombstone {
		t.Fatalf("expected tombstone to be true")
	}
}

func TestHashmapFlush(t *testing.T) {
	t.Log("---- HASHMAP MEMTABLE FLUSH TEST ----")
	flushed := [][]MemtableEntry{}
	mt := NewMemtables("hashmap", 1, 2, func(entries []MemtableEntry) {
		flushed = append(flushed, entries)
	})
	mt.Put("key1", []byte("value1"))
	mt.Put("key2", []byte("value2"))
	mt.Put("key3", []byte("value3"))
	_, ok := mt.Get("key1")
	if ok {
		t.Fatalf("expected key1 to be missing after flush")
	}
	_, ok = mt.Get("key3")
	if !ok {
		t.Fatalf("expected key3 to exist")
	}
	if len(flushed) != 1 {
		t.Fatalf("expected 1 flush, got %d", len(flushed))
	}
	if len(flushed[0]) != 2 {
		t.Fatalf("expected 2 flushed entries, got %d", len(flushed[0]))
	}
}

func TestHashmapRotation(t *testing.T) {
	t.Log("---- HASHMAP MEMTABLE ROTATION TEST ----")
	mt := NewMemtables("hashmap", 2, 2, func(entries []MemtableEntry) {})

	mt.Put("key1", []byte("value1"))
	mt.Put("key2", []byte("value2"))
	mt.Put("key3", []byte("value3"))

	entries := mt.ReadEntriesNoFlushing()
	foundKeys := map[string]bool{}
	for _, e := range entries {
		foundKeys[e.Key] = true
	}
	if !foundKeys["key1"] || !foundKeys["key2"] || !foundKeys["key3"] {
		t.Fatalf("expected keys key1, key2, key3 to exist in memtables")
	}
	if mt.activeIndex != 1 {
		t.Fatalf("active index expected to be 1 because of rotation, got %d", mt.activeIndex)
	}
	entry, ok := mt.Get("key1")
	if !ok || string(entry.Value) != "value1" {
		t.Fatalf("expected key1 to have value1, got %v", entry.Value)
	}
}

func TestSkipList(t *testing.T) {
	t.Log("---- SKIPLIST MEMTABLE TEST ----")
	mt := NewMemtables("skiplist", 1, 100, nil)
	mt.Put("key1", []byte("value1"))
	mt.Put("key2", []byte("value2"))
	mt.Put("key3", []byte("value3"))
	mt.Put("key4", []byte("value4"))
	entry, ok := mt.Get("key1")
	if !ok {
		t.Fatal("key1 not found, expected to be found")
	}
	if string(entry.Value) != "value1" {
		t.Fatalf("expected value1, got %s", entry.Value)
	}
	mt.Put("key1", []byte("value1_updated"))
	entryUpdated, ok := mt.Get("key1")
	if !ok {
		t.Fatalf("key1 not found")
	}
	if string(entryUpdated.Value) != "value1_updated" {
		t.Fatalf("expected value1_updated, got %s", entryUpdated.Value)
	}
	_, ok = mt.Get("nonexisting")
	if ok {
		t.Fatalf("expected nonexisting key to be missing")
	}
	mt.Delete("nonexisting")
	entries := mt.ReadEntriesNoFlushing()
	if !entries[4].Tombstone {
		t.Fatalf("expected tombstone to be true")
	}
}

func TestSkipListFlush(t *testing.T) {
	t.Log("---- SKIPLIST MEMTABLE FLUSH TEST ----")
	flushed := [][]MemtableEntry{}
	mt := NewMemtables("skiplist", 1, 2, func(entries []MemtableEntry) {
		flushed = append(flushed, entries)
	})
	mt.Put("key1", []byte("value1"))
	mt.Put("key2", []byte("value2"))
	mt.Put("key3", []byte("value3"))
	_, ok := mt.Get("key1")
	if ok {
		t.Fatalf("expected key1 to be missing after flush")
	}
	_, ok = mt.Get("key3")
	if !ok {
		t.Fatalf("expected key3 to exist")
	}
	if len(flushed) != 1 {
		t.Fatalf("expected 1 flush, got %d", len(flushed))
	}
	if len(flushed[0]) != 2 {
		t.Fatalf("expected 2 flushed entries, got %d", len(flushed[0]))
	}
}

func TestSkipListRotation(t *testing.T) {
	t.Log("---- HASHMAP MEMTABLE ROTATION TEST ----")
	mt := NewMemtables("skiplist", 2, 2, func(entries []MemtableEntry) {})
	mt.Put("key1", []byte("value1"))
	mt.Put("key2", []byte("value2"))
	mt.Put("key3", []byte("value3"))

	entries := mt.ReadEntriesNoFlushing()
	foundKeys := map[string]bool{}
	for _, e := range entries {
		foundKeys[e.Key] = true
	}
	if !foundKeys["key1"] || !foundKeys["key2"] || !foundKeys["key3"] {
		t.Fatalf("expected keys key1, key2, key3 to exist in memtables")
	}
	if mt.activeIndex != 1 {
		t.Fatalf("active index expected to be 1 because of rotation, got %d", mt.activeIndex)
	}
	entry, ok := mt.Get("key1")
	if !ok || string(entry.Value) != "value1" {
		t.Fatalf("expected key1 to have value1, got %v", entry.Value)
	}
}

func TestBTree(t *testing.T) {
	t.Log("---- BTREE MEMTABLE TEST ----")
	mt := NewMemtables("btree", 1, 5, func(entries []MemtableEntry) {})
	mt.Put("key2", []byte("value2"))
	mt.Put("key3", []byte("value3"))
	mt.Put("key4", []byte("value4"))
	mt.Put("key6", []byte("value6"))
	mt.Put("key5", []byte("value5"))
	mt.Put("key1", []byte("value1"))
	entry, ok := mt.Get("key1")
	if !ok {
		t.Fatal("expected key1 to be found")
	}
	if string(entry.Value) != "value1" {
		t.Fatalf("expected value1 but got %s", string(entry.Value))
	}
	mt.Put("key1", []byte("value1_updated"))
	entryUpdated, ok := mt.Get("key1")
	if !ok {
		t.Fatal("expected key1 to be found")
	}
	if string(entryUpdated.Value) != "value1_updated" {
		t.Fatalf("expected value1_updated but got %s", string(entryUpdated.Value))
	}
}

func TestBTreeFlush(t *testing.T) {
	t.Log("---- BTREE MEMTABLE FLUSH TEST ----")
	flushed := [][]MemtableEntry{}
	mt := NewMemtables("btree", 1, 2, func(entries []MemtableEntry) {
		flushed = append(flushed, entries)
	})
	mt.Put("key1", []byte("value1"))
	mt.Put("key2", []byte("value2"))
	mt.Put("key3", []byte("value3"))
	_, ok := mt.Get("key1")
	if ok {
		t.Fatalf("expected key1 to be missing after flush")
	}
	_, ok = mt.Get("key3")
	if !ok {
		t.Fatalf("expected key3 to exist")
	}
	if len(flushed) != 1 {
		t.Fatalf("expected 1 flush, got %d", len(flushed))
	}
	if len(flushed[0]) != 2 {
		t.Fatalf("expected 2 flushed entries, got %d", len(flushed[0]))
	}
}

func TestBTreeRotation(t *testing.T) {
	t.Log("---- HASHMAP MEMTABLE ROTATION TEST ----")
	mt := NewMemtables("skiplist", 2, 2, func(entries []MemtableEntry) {})
	mt.Put("key1", []byte("value1"))
	mt.Put("key2", []byte("value2"))
	mt.Put("key3", []byte("value3"))

	entries := mt.ReadEntriesNoFlushing()
	foundKeys := map[string]bool{}
	for _, e := range entries {
		foundKeys[e.Key] = true
	}
	if !foundKeys["key1"] || !foundKeys["key2"] || !foundKeys["key3"] {
		t.Fatalf("expected keys key1, key2, key3 to exist in memtables")
	}
	if mt.activeIndex != 1 {
		t.Fatalf("active index expected to be 1 because of rotation, got %d", mt.activeIndex)
	}
	entry, ok := mt.Get("key1")
	if !ok || string(entry.Value) != "value1" {
		t.Fatalf("expected key1 to have value1, got %v", entry.Value)
	}
}
