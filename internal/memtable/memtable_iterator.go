package memtable

import (
	"bytes"

	"github.com/ajromen/LSM-KV-Engine/internal/data_structures"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
	"github.com/ajromen/LSM-KV-Engine/internal/iterator"
)

// this file implements the iterator layer for memtables
// iterators are one of the most important parts of this engine
// all iterators have two states -> raw and filtering

// RawSingleMemtableIterator wraps a single memtable iterator without deduplication and tombstone handling
// it does not modify behavior, only delegates calls
type RawSingleMemtableIterator struct {
	it iterator.Iterator[MemtableEntry]
}

// NewRawSingleMemtableIterator creates a new raw iterator for a single memtable
func NewRawSingleMemtableIterator(it iterator.Iterator[MemtableEntry]) iterator.Iterator[MemtableEntry] {
	return &RawSingleMemtableIterator{
		it: it,
	}
}

// Valid returns whether the iterator is currently pointing to a valid element
func (r *RawSingleMemtableIterator) Valid() bool {
	return r.it.Valid()
}

// SeekToFirst moves the iterator to the first element (smallest entry)
func (r *RawSingleMemtableIterator) SeekToFirst() {
	r.it.SeekToFirst()
}

// SeekToLast moves the iterator to the last element (largest entry)
func (r *RawSingleMemtableIterator) SeekToLast() {
	r.it.SeekToLast()
}

// Seek moves the iterator to the first element >= key
func (r *RawSingleMemtableIterator) Seek(key MemtableEntry) {
	r.it.Seek(key)
}

// Next moves iterator forward
func (r *RawSingleMemtableIterator) Next() {
	r.it.Next()
}

// Prev moves iterator backward
func (r *RawSingleMemtableIterator) Prev() {
	r.it.Prev()
}

// Key returns current key-value entry
func (r *RawSingleMemtableIterator) Key() MemtableEntry {
	return r.it.Key()
}

// Value returns current key-value entry
func (r *RawSingleMemtableIterator) Value() MemtableEntry {
	return r.it.Value()
}

// SingleMemtableIterator adds lsm semantics on top of a raw single memtable iterator
// lsm semantics -> filtering keys -> removing duplicates and skipping tombstones
type SingleMemtableIterator struct {
	rawIt   *RawSingleMemtableIterator
	current *MemtableEntry
	valid   bool
}

// NewSingleMemtableIterator creates a cleaned iterator over a single memtable
func NewSingleMemtableIterator(rawIt *RawSingleMemtableIterator) iterator.Iterator[MemtableEntry] {
	it := &SingleMemtableIterator{
		rawIt: rawIt,
	}
	it.SeekToFirst()
	return it
}

// Valid returns whether the iterator is currently pointing to a valid element
func (s *SingleMemtableIterator) Valid() bool {
	return s.valid
}

// SeekToFirst moves to first valid (non-deleted, non-duplicated) entry
func (s *SingleMemtableIterator) SeekToFirst() {
	s.rawIt.SeekToFirst()
	s.advanceToNextUnique(nil)
}

// SeekToLast not implemented
func (s *SingleMemtableIterator) SeekToLast() {
	panic("SeekToLast not implemented")
}

// Seek moves the iterator to first valid entry >= key
func (s *SingleMemtableIterator) Seek(key MemtableEntry) {
	s.rawIt.Seek(key)
	s.advanceToNextUnique(nil)
}

// Next advances while taking care of deduplication and tombstones.
func (s *SingleMemtableIterator) Next() {
	if !s.valid {
		return
	}
	prevKey := s.current.Key
	s.rawIt.Next()
	s.advanceToNextUnique(prevKey)
}

// Prev not implemented
func (s *SingleMemtableIterator) Prev() {
	panic("Prev not implemented")
}

// Key returns current entry.
func (s *SingleMemtableIterator) Key() MemtableEntry {
	return *s.current
}

// Value returns current entry.
func (s *SingleMemtableIterator) Value() MemtableEntry {
	return *s.current
}

// advanceToNextUnique makes LSM rules possible:
// 1. skip duplicates (same key -> different versions)
// 2. skip tombstones (if first version has tombstone=true skip key completely)
func (s *SingleMemtableIterator) advanceToNextUnique(prevKey []byte) {
	for s.rawIt.Valid() {
		entry := s.rawIt.Value()
		// 1. skip duplicates
		if prevKey != nil && bytes.Equal(entry.Key, prevKey) {
			s.rawIt.Next()
			continue
		}
		// 2. skip tombstones
		if entry.Tombstone {
			prevKey = entry.Key
			s.rawIt.Next()
			continue
		}
		s.current = &entry
		s.valid = true
		return
	}
	s.current = nil
	s.valid = false
}

// RawIterator merges multiple memtable iterators into a single iterator with a single sorted stream of entries
// It uses a MergeStructure (Heap or WinnerTree) for ordering
// no filtering -> raw-level
type RawIterator struct {
	structure data_structures.MergeStructure[MemtableEntry]
	current   *MemtableEntry
	valid     bool
}

// NewRawIterator creates a merged iterator from multiple single memtable iterators (could be said multiple memtables)
func NewRawIterator(iterators []iterator.Iterator[MemtableEntry], mergeStructure byte) *RawIterator {
	if len(iterators) == 0 {
		return &RawIterator{valid: false}
	}
	active := []iterator.Iterator[MemtableEntry]{}
	for _, it := range iterators {
		if it != nil && it.Valid() {
			active = append(active, it)
		}
	}
	if len(active) == 0 {
		return &RawIterator{valid: false}
	}
	cmp := func(a, b MemtableEntry) int {
		if c := bytes.Compare(a.Key, b.Key); c != 0 {
			return c
		}
		if a.SeqId > b.SeqId {
			return -1
		}
		if a.SeqId < b.SeqId {
			return 1
		}
		return 0
	}
	structure := data_structures.NewMergeStructure(mergeStructure, active, cmp)
	it := &RawIterator{
		structure: structure,
	}
	winner := structure.Winner()
	if winner != nil && winner.Valid() {
		entry := winner.Key()
		it.current = &entry
		it.valid = true
	}
	return it
}

// Valid returns whether the iterator is currently pointing to a valid element
func (i *RawIterator) Valid() bool {
	if i.structure == nil {
		i.valid = false
		return false
	}
	return i.valid
}

// SeekToFirst initializes all iterators and positions them on first element and then finds first global element
// first global element is the smallest one of all entries on which all iterators currently point to
func (i *RawIterator) SeekToFirst() {
	if i.structure == nil {
		i.valid = false
		return
	}
	for _, it := range i.structure.Iterators() {
		it.SeekToFirst()
		i.structure.Update(it)
	}
	winner := i.structure.Winner()
	if winner != nil && winner.Valid() {
		entry := winner.Key()
		i.current = &entry
		i.valid = true
	} else {
		i.current = nil
		i.valid = false
	}
}

func reverseComparator(cmp func(a, b MemtableEntry) int) func(a, b MemtableEntry) int {
	return func(a, b MemtableEntry) int {
		return -cmp(a, b)
	}
}

// SeekToLast finds last element using reversed comparator trick
// not used hence why bad performance
func (i *RawIterator) SeekToLast() {
	for _, it := range i.structure.Iterators() {
		it.SeekToLast()
	}
	reverseCmp := reverseComparator(i.structure.Cmp())
	tempStructure := data_structures.NewMergeStructure(byte(enums.Heap), i.structure.Iterators(), reverseCmp)
	winner := tempStructure.Winner()
	if winner != nil && winner.Valid() {
		entry := winner.Key()
		i.current = &entry
		i.valid = true
	} else {
		i.current = nil
		i.valid = false
	}
}

// Seek positions all iterators at key and recomputes winner
func (i *RawIterator) Seek(key MemtableEntry) {
	for _, it := range i.structure.Iterators() {
		it.Seek(key)
	}
	for _, it := range i.structure.Iterators() {
		i.structure.Update(it)
	}
	winner := i.structure.Winner()
	if winner != nil && winner.Valid() {
		entry := winner.Key()
		i.current = &entry
		i.valid = true
	} else {
		i.current = nil
		i.valid = false
	}
}

// Next advances global merge heap or winner tree
func (i *RawIterator) Next() {
	if !i.valid {
		return
	}
	winner := i.structure.Winner()
	if winner == nil {
		i.current = nil
		i.valid = false
		return
	}
	i.structure.Advance(winner)
	winner = i.structure.Winner()
	if winner != nil && winner.Valid() {
		entry := winner.Key()
		i.current = &entry
		i.valid = true
	} else {
		i.current = nil
		i.valid = false
	}
}

// Prev not fully supported; uses reverse merge trick
func (i *RawIterator) Prev() {
	if !i.valid {
		return
	}
	reverseCmp := reverseComparator(i.structure.Cmp())
	tempStructure := data_structures.NewMergeStructure(byte(enums.Heap), i.structure.Iterators(), reverseCmp)
	winner := tempStructure.Winner()
	if winner != nil && winner.Valid() {
		entry := winner.Key()
		i.current = &entry
		i.valid = true
	} else {
		i.current = nil
		i.valid = false
	}
}

// Key returns current entry
func (i *RawIterator) Key() MemtableEntry {
	return *i.current
}

// Value returns current entry
func (i *RawIterator) Value() MemtableEntry {
	return *i.current
}

// MergedMemtableIterator -> final lsm-level iterator linked to memtable L0
// removes tombstones and duplicates across all instances of memtables
// when we want all entries without duplicates and deleted versions we just iterate over memtables using this iterator
// one of the most important parts of the system
type MergedMemtableIterator struct {
	rawIterator *RawIterator
	current     *MemtableEntry
	prevKey     []byte
	valid       bool
}

func NewMergedMemtableIterator(rawIterator *RawIterator) iterator.Iterator[MemtableEntry] {
	it := &MergedMemtableIterator{
		rawIterator: rawIterator,
	}
	it.SeekToFirst()
	return it
}

func (m *MergedMemtableIterator) Valid() bool {
	return m.valid
}

func (m *MergedMemtableIterator) Key() MemtableEntry {
	return *m.current
}

func (m *MergedMemtableIterator) Value() MemtableEntry {
	return *m.current
}

// SeekToFirst starts from beginning of merged view
func (m *MergedMemtableIterator) SeekToFirst() {
	if m.rawIterator.structure == nil {
		m.valid = false
		return
	}
	m.prevKey = nil
	m.rawIterator.SeekToFirst()
	m.advance()
}

// SeekToLast starts from end (best-effort)
func (m *MergedMemtableIterator) SeekToLast() {
	m.prevKey = nil
	m.rawIterator.SeekToLast()
	m.advance()
}

// Seek positions iterator at key
func (m *MergedMemtableIterator) Seek(key MemtableEntry) {
	m.prevKey = nil
	m.rawIterator.Seek(key)
	m.advance()
}

// Next moves forward with LSM rules applied
func (m *MergedMemtableIterator) Next() {
	if !m.valid {
		return
	}
	m.prevKey = m.current.Key
	m.rawIterator.Next()
	m.advance()
}

// Prev not implemented
func (m *MergedMemtableIterator) Prev() {
	panic("Prev not implemented")
}

// advance applies final LSM filtering
func (m *MergedMemtableIterator) advance() {
	for m.rawIterator.Valid() {
		entry := m.rawIterator.Key()
		if entry.Tombstone {
			m.rawIterator.Next()
			continue
		}
		if m.prevKey != nil && bytes.Equal(entry.Key, m.prevKey) {
			m.rawIterator.Next()
			continue
		}
		m.current = &entry
		m.valid = true
		return
	}
	m.current = nil
	m.valid = false
}
