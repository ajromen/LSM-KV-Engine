package memtable

import (
	"github.com/ajromen/LSM-KV-Engine/internal/data_structures"
	"github.com/ajromen/LSM-KV-Engine/internal/iterator"
)

// Memtable Store Adapters -> set of adapter types that wrap different in-memory data structures for storing memtable entries
// Adapters provide an interface for basic operations over those in-memory data structures and provide generic look on memtable itself
// Adapters allow the memtable layer to remain data-structure-agnostic,so different in-memory structures can be swapped without changing the upper layers.
// All adapters must implement MemtableStore interface

// ---- BTree Store Adapter ----

type BTreeStore struct {
	tree *data_structures.BTree[MemtableEntry]
}

func NewBTreeStore(t int, cmp data_structures.Comparator[MemtableEntry]) *BTreeStore {
	return &BTreeStore{tree: data_structures.NewBTree[MemtableEntry](t, cmp)}
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
	return NewSingleMemtableIterator(rawIt)
}

// ---- Skiplist store adapter ----

type SkipListStore struct {
	list *data_structures.SkipList[MemtableEntry]
}

func NewSkipListStore(maxLevel int, cmp data_structures.Comparator[MemtableEntry]) *SkipListStore {
	return &SkipListStore{list: data_structures.NewSkipList[MemtableEntry](maxLevel, cmp)}
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
	return NewSingleMemtableIterator(rawIt)
}

// ---- Hashmap store adapter ----

type HashMapStore struct {
	hmap *data_structures.HackMap[MemtableEntry]
}

func NewHashMapStore() *HashMapStore {
	return &HashMapStore{
		hmap: data_structures.NewHackMap[MemtableEntry](),
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
	s.hmap = data_structures.NewHackMap[MemtableEntry]()
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
	return NewSingleMemtableIterator(rawIt)
}

// ---- RBTree store adapter ----

type RBTreeStore struct {
	tree *data_structures.RBTree[MemtableEntry]
}

func NewRBTreeStore(
	cmp data_structures.Comparator[MemtableEntry],
	cmpIgnoringSeqId data_structures.Comparator[MemtableEntry],
) *RBTreeStore {
	return &RBTreeStore{
		tree: data_structures.NewRBTree[MemtableEntry](cmp, cmpIgnoringSeqId),
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
	return NewSingleMemtableIterator(rawIt)
}

// ---- AVLTree store adapter ----

type AVLTreeStore struct {
	tree *data_structures.AVLTree[MemtableEntry]
}

func NewAVLTreeStore(
	cmp data_structures.Comparator[MemtableEntry],
	cmpIgnoringSeqId data_structures.Comparator[MemtableEntry],
) *AVLTreeStore {
	return &AVLTreeStore{
		tree: data_structures.NewAVLTree[MemtableEntry](cmp, cmpIgnoringSeqId),
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
	return NewSingleMemtableIterator(rawIt)
}
