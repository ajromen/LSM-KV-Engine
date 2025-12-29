package memtable

import (
	"testing"
)

func TestHashmap(t *testing.T) {
	t.Log("---- HASHMAP MEMTABLE TEST ----\n")
	hmt := NewMemtable("hashmap", 100, func(entries []MemtableEntry) {})
	hmt.Put("key1", []byte("value1"))
	hmt.Put("key2", []byte("value2"))
	hmt.Put("key3", []byte("value3"))
	entry, ok := hmt.Get("key1")
	if !ok {
		t.Fatal("key1 not found")
	}
	if string(entry.Value) != "value1" {
		t.Fatalf("expected value1, got %s", entry.Value)
	}
	hmt.Put("key1", []byte("value1_updated"))
	entryUpdated, ok := hmt.Get("key1")
	if !ok {
		t.Fatalf("key1 not found")
	}
	if string(entryUpdated.Value) != "value1_updated" {
		t.Fatalf("expected value1_updated, got %s", entryUpdated.Value)
	}
	_, ok = hmt.Get("nonexisting")
	if ok {
		t.Fatalf("expected nonexisting key to be missing")
	}
	hmt.Put("key0", []byte("value0"))
	entries := hmt.FlushEntries()
	if entries[0].Key != "key0" {
		t.Fatalf("expected key0 to be first in flush, but first is %s", entries[0].Key)
	}
	_, ok = hmt.Get("key0")
	if ok {
		t.Fatalf("expected key0 to be missing because it was flushed")
	}
	flushed := [][]MemtableEntry{}
	hmtTestFlush := NewHashMap(2, func(entries []MemtableEntry) {
		flushed = append(flushed, entries)
	})
	hmtTestFlush.Put("key1", []byte("value1"))
	hmtTestFlush.Put("key2", []byte("value2"))
	hmtTestFlush.Put("key3", []byte("value3"))
	_, ok = hmtTestFlush.Get("key1")
	if ok {
		t.Fatalf("expected key1 to be missing because it shouldve flushed")
	}
	_, ok = hmtTestFlush.Get("key3")
	if !ok {
		t.Fatalf("expected key3 to be present in a new HashMapMemtable after the flush")
	}
	if len(flushed) != 1 {
		t.Fatalf("expected 1 flush, got %d", len(flushed))
	}
	if len(flushed[0]) != 2 {
		t.Fatalf("expected 2 entries in flush, got %d", len(flushed[0]))
	}
	succ := hmtTestFlush.Delete("key3")
	if !succ {
		t.Fatalf("shouldve been succesful because key3 really exists, but it was unsuccesful")
	}
	succ_unsucc := hmtTestFlush.Delete("key4")
	if succ_unsucc {
		t.Fatalf("shouldve been unsuccesful because key4 doesnt exists, but it was succesful")
	}
	entriesNoFlush := hmtTestFlush.ReadEntriesNoFlushing()
	if len(entriesNoFlush) != 1 {
		t.Fatalf("should be 1 element, but there is %d", len(entriesNoFlush))
	}
	if entriesNoFlush[0].Tombstone == false {
		t.Fatalf("tombstone expected to be true, but is false")
	}
}

func TestSkipList(t *testing.T) {
	t.Log("---- SKIPLIST MEMTABLE TEST ----")
	smt := NewMemtable("skiplist", 100, func(entries []MemtableEntry) {})
	smt.Put("key1", []byte("value1"))
	smt.Put("key2", []byte("value2"))
	smt.Put("key3", []byte("value3"))
	entry, ok := smt.Get("key1")
	if !ok {
		t.Fatal("key1 not found")
	}
	if string(entry.Value) != "value1" {
		t.Fatalf("expected value1, got %s", string(entry.Value))
	}
}
