package memtable

import (
	"bytes"
	"math"

	"github.com/ajromen/LSM-KV-Engine/internal/iterator"
	"github.com/ajromen/LSM-KV-Engine/internal/structures"
)

// Memtable Store Adapters -> set of adapter types that wrap different in-memory data structures for storing memtable entries
// Adapters provide an interface for basic operations over those in-memory data structures and provide generic look on memtable itself
// Adapters allow the memtable layer to remain data-structure-agnostic,so different in-memory structures can be swapped without changing the upper layers.
// All adapters must implement MemtableStore interface

// ---- BTree Store Adapter ----

type BTreeStore struct {
	tree *structures.BTree[MemtableEntry]
}

func NewBTreeStore(t int, cmp structures.Comparator[MemtableEntry]) *BTreeStore {
	return &BTreeStore{tree: structures.NewBTree[MemtableEntry](t, cmp)}
}

func (s *BTreeStore) Insert(entry MemtableEntry) {
	s.tree.Insert(entry)
}

func (s *BTreeStore) Search(entry MemtableEntry) *MemtableEntry {
	result, found := s.tree.LowerBound(entry)
	if !found {
		return nil
	}
	return &result
}

func (s *BTreeStore) EntriesInOrder() []MemtableEntry {
	return s.tree.EntriesInOrder()
}

func (s *BTreeStore) Reset() {
	s.tree.Reset()
}

func (s *BTreeStore) Size() int {
	return s.tree.Size()
}

func (s *BTreeStore) Visualize(formatter func(MemtableEntry) string) string {
	return s.tree.Visualize(formatter)
}

func (s *BTreeStore) RawIterator() iterator.Iterator[MemtableEntry] {
	return NewRawSingleMemtableIterator(s.tree.Iterator())
}

func (s *BTreeStore) Iterator() iterator.Iterator[MemtableEntry] {
	rawIt := s.RawIterator().(*RawSingleMemtableIterator)
	return NewSingleMemtableIterator(rawIt, nil)
}

func (s *BTreeStore) Upsert(entry MemtableEntry) bool {
	dummy := MemtableEntry{Key: entry.Key, SeqId: math.MaxUint64}
	existing, found := s.tree.LowerBound(dummy)
	replaced := found && bytes.Equal(existing.Key, entry.Key)
	if replaced {
		s.tree.Delete(existing)
	}
	s.tree.Insert(entry)
	return replaced
}

func (s *BTreeStore) Remove(entry MemtableEntry) {
	s.tree.Delete(entry)
}

// ---- Skiplist store adapter ----

type SkipListStore struct {
	list *structures.SkipList[MemtableEntry]
}

func NewSkipListStore(maxLevel int, cmp structures.Comparator[MemtableEntry]) *SkipListStore {
	return &SkipListStore{list: structures.NewSkipList[MemtableEntry](maxLevel, cmp)}
}

func (s *SkipListStore) Insert(entry MemtableEntry) {
	s.list.Insert(entry)
}

func (s *SkipListStore) EntriesInOrder() []MemtableEntry {
	return s.list.EntriesInOrder()
}

func (s *SkipListStore) Reset() {
	s.list.Reset()
}

func (s *SkipListStore) Size() int {
	return s.list.Size()
}

func (s *SkipListStore) Visualize(f func(MemtableEntry) string) string {
	return s.list.Visualize(f)
}

func (s *SkipListStore) Search(entry MemtableEntry) *MemtableEntry {
	result, found := s.list.LowerBound(entry)
	if !found {
		return nil
	}
	return &result
}

func (s *SkipListStore) RawIterator() iterator.Iterator[MemtableEntry] {
	return NewRawSingleMemtableIterator(s.list.Iterator())
}

func (s *SkipListStore) Iterator() iterator.Iterator[MemtableEntry] {
	rawIt := s.RawIterator().(*RawSingleMemtableIterator)
	return NewSingleMemtableIterator(rawIt, nil)
}

func (s *SkipListStore) Upsert(entry MemtableEntry) bool {
	dummy := MemtableEntry{Key: entry.Key, SeqId: math.MaxUint64}
	existing, found := s.list.LowerBound(dummy)
	replaced := found && bytes.Equal(existing.Key, entry.Key)
	if replaced {
		s.list.Delete(existing)
	}
	s.list.Insert(entry)
	return replaced
}

func (s *SkipListStore) Remove(entry MemtableEntry) {
	s.list.Delete(entry)
}

// ---- Hashmap store adapter ----

type HashMapStore struct {
	hmap *structures.HackMap[MemtableEntry]
}

func NewHashMapStore() *HashMapStore {
	return &HashMapStore{
		hmap: structures.NewHackMap[MemtableEntry](),
	}
}

func (s *HashMapStore) Insert(entry MemtableEntry) {
	s.hmap.Insert(entry)
}

func (s *HashMapStore) Search(entry MemtableEntry) *MemtableEntry {
	node, found := s.hmap.LowerBound(string(entry.Key))
	if found {
		return &node
	}
	return nil
}

func (s *HashMapStore) Remove(entry MemtableEntry) {
	s.hmap.Remove(entry)
}

func (s *HashMapStore) EntriesInOrder() []MemtableEntry {
	return s.hmap.EntriesInOrder()
}

func (s *HashMapStore) Reset() {
	s.hmap = structures.NewHackMap[MemtableEntry]()
}

func (s *HashMapStore) Size() int {
	total := 0
	for _, stack := range s.hmap.Data() {
		total += stack.Len()
	}
	return total
}

func (s *HashMapStore) Visualize(f func(MemtableEntry) string) string {
	return s.hmap.Visualize(f)
}

func (s *HashMapStore) RawIterator() iterator.Iterator[MemtableEntry] {
	return NewRawSingleMemtableIterator(s.hmap.Iterator())
}

func (s *HashMapStore) Iterator() iterator.Iterator[MemtableEntry] {
	rawIt := s.RawIterator().(*RawSingleMemtableIterator)
	return NewSingleMemtableIterator(rawIt, nil)
}

func (s *HashMapStore) Upsert(entry MemtableEntry) bool {
	return s.hmap.Upsert(entry)
}

// ---- RBTree store adapter ----

type RBTreeStore struct {
	tree *structures.RBTree[MemtableEntry]
}

func NewRBTreeStore(
	cmp structures.Comparator[MemtableEntry],
	cmpIgnoringSeqId structures.Comparator[MemtableEntry],
) *RBTreeStore {
	return &RBTreeStore{
		tree: structures.NewRBTree[MemtableEntry](cmp, cmpIgnoringSeqId),
	}
}

func (s *RBTreeStore) Insert(entry MemtableEntry) {
	s.tree.Insert(entry)
}

func (s *RBTreeStore) Search(entry MemtableEntry) *MemtableEntry {
	node := s.tree.LowerBound(entry)
	if node == nil {
		return nil
	}

	val := node.Key
	return &val
}

func (s *RBTreeStore) EntriesInOrder() []MemtableEntry {
	return s.tree.EntriesInOrder()
}

func (s *RBTreeStore) Reset() {
	s.tree.Reset()
}

func (s *RBTreeStore) Size() int {
	return s.tree.Size()
}

func (s *RBTreeStore) Visualize(formatter func(MemtableEntry) string) string {
	return s.tree.Visualize(formatter)
}

func (s *RBTreeStore) RawIterator() iterator.Iterator[MemtableEntry] {
	return NewRawSingleMemtableIterator(s.tree.Iterator())
}

func (s *RBTreeStore) Iterator() iterator.Iterator[MemtableEntry] {
	rawIt := s.RawIterator().(*RawSingleMemtableIterator)
	return NewSingleMemtableIterator(rawIt, nil)
}

func (s *RBTreeStore) Upsert(entry MemtableEntry) bool {
	dummy := MemtableEntry{Key: entry.Key, SeqId: math.MaxUint64}
	node := s.tree.LowerBound(dummy)
	replaced := node != nil && bytes.Equal(node.Key.Key, entry.Key)
	if replaced {
		s.tree.Delete(node)
	}
	s.tree.Insert(entry)
	return replaced
}

func (s *RBTreeStore) Remove(entry MemtableEntry) {
	node := s.tree.LowerBound(entry)
	if node != nil && bytes.Equal(node.Key.Key, entry.Key) && node.Key.SeqId == entry.SeqId {
		s.tree.Delete(node)
	}
}

// ---- AVLTree store adapter ----

type AVLTreeStore struct {
	tree *structures.AVLTree[MemtableEntry]
}

func NewAVLTreeStore(
	cmp structures.Comparator[MemtableEntry],
	cmpIgnoringSeqId structures.Comparator[MemtableEntry],
) *AVLTreeStore {
	return &AVLTreeStore{
		tree: structures.NewAVLTree[MemtableEntry](cmp, cmpIgnoringSeqId),
	}
}

func (s *AVLTreeStore) Insert(entry MemtableEntry) {
	s.tree.Insert(entry)
}

func (s *AVLTreeStore) Search(entry MemtableEntry) *MemtableEntry {
	node := s.tree.LowerBound(entry)
	if node == nil {
		return nil
	}
	val := node.Key
	return &val
}

func (s *AVLTreeStore) EntriesInOrder() []MemtableEntry {
	return s.tree.EntriesInOrder()
}

func (s *AVLTreeStore) Reset() {
	s.tree.Reset()
}

func (s *AVLTreeStore) Size() int {
	return s.tree.Size()
}

func (s *AVLTreeStore) Visualize(formatter func(MemtableEntry) string) string {
	return s.tree.Visualize(formatter)
}

func (s *AVLTreeStore) RawIterator() iterator.Iterator[MemtableEntry] {
	return NewRawSingleMemtableIterator(s.tree.Iterator())
}

func (s *AVLTreeStore) Iterator() iterator.Iterator[MemtableEntry] {
	rawIt := s.RawIterator().(*RawSingleMemtableIterator)
	return NewSingleMemtableIterator(rawIt, nil)
}

func (s *AVLTreeStore) Upsert(entry MemtableEntry) bool {
	dummy := MemtableEntry{Key: entry.Key, SeqId: math.MaxUint64}
	node := s.tree.LowerBound(dummy)
	replaced := node != nil && bytes.Equal(node.Key.Key, entry.Key)
	if replaced {
		s.tree.Delete(node.Key)
	}
	s.tree.Insert(entry)
	return replaced
}

func (s *AVLTreeStore) Remove(entry MemtableEntry) {
	node := s.tree.LowerBound(entry)
	if node != nil && bytes.Equal(node.Key.Key, entry.Key) && node.Key.SeqId == entry.SeqId {
		s.tree.Delete(node.Key)
	}
}
