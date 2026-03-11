package memtable

import (
	"bytes"

	"github.com/ajromen/LSM-KV-Engine/internal/data_structures"
	"github.com/ajromen/LSM-KV-Engine/internal/include"
)

type RawSingleMemtableIterator struct {
	it include.Iterator[MemtableEntry]
}

func NewRawSingleMemtableIterator(it include.Iterator[MemtableEntry]) include.Iterator[MemtableEntry] {
	return &RawSingleMemtableIterator{
		it: it,
	}
}

func (r *RawSingleMemtableIterator) Valid() bool {
	return r.it.Valid()
}

func (r *RawSingleMemtableIterator) SeekToFirst() {
	r.it.SeekToFirst()
}

func (r *RawSingleMemtableIterator) SeekToLast() {
	r.it.SeekToLast()
}

func (r *RawSingleMemtableIterator) Seek(key MemtableEntry) {
	r.it.Seek(key)
}

func (r *RawSingleMemtableIterator) Next() {
	r.it.Next()
}

func (r *RawSingleMemtableIterator) Prev() {
	r.it.Prev()
}

func (r *RawSingleMemtableIterator) Key() MemtableEntry {
	return r.it.Key()
}

func (r *RawSingleMemtableIterator) Value() MemtableEntry {
	return r.it.Value()
}

type SingleMemtableIterator struct {
	rawIt   *RawSingleMemtableIterator
	current *MemtableEntry
	valid   bool
}

func NewSingleMemtableIterator(rawIt *RawSingleMemtableIterator) include.Iterator[MemtableEntry] {
	it := &SingleMemtableIterator{
		rawIt: rawIt,
	}
	it.SeekToFirst()
	return it
}

func (s *SingleMemtableIterator) Valid() bool {
	return s.valid
}

func (s *SingleMemtableIterator) SeekToFirst() {
	s.rawIt.SeekToFirst()
	s.advanceToNextUnique(nil)
}

func (s *SingleMemtableIterator) SeekToLast() {
	panic("SeekToLast not implemented")
}

func (s *SingleMemtableIterator) Seek(key MemtableEntry) {
	s.rawIt.Seek(key)
	s.advanceToNextUnique(nil)
}

func (s *SingleMemtableIterator) Next() {
	if !s.valid {
		return
	}
	prevKey := s.current.Key
	s.rawIt.Next()
	s.advanceToNextUnique(prevKey)
}

func (s *SingleMemtableIterator) Prev() {
	panic("Prev not implemented")
}

func (s *SingleMemtableIterator) Key() MemtableEntry {
	return *s.current
}

func (s *SingleMemtableIterator) Value() MemtableEntry {
	return *s.current
}

func (s *SingleMemtableIterator) advanceToNextUnique(prevKey []byte) {
	for s.rawIt.Valid() {
		entry := s.rawIt.Value()
		if prevKey != nil && bytes.Equal(entry.Key, prevKey) {
			s.rawIt.Next()
			continue
		}
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

type RawIterator struct {
	structure data_structures.MergeStructure[MemtableEntry]
	current   *MemtableEntry
	valid     bool
}

func NewRawIterator(iterators []include.Iterator[MemtableEntry], mergeStructure byte) *RawIterator {
	if len(iterators) == 0 {
		return &RawIterator{valid: false}
	}
	active := []include.Iterator[MemtableEntry]{}
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
		if a.Timestamp > b.Timestamp {
			return -1
		}
		if a.Timestamp < b.Timestamp {
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

func (i *RawIterator) Valid() bool {
	if i.structure == nil {
		i.valid = false
		return false
	}
	return i.valid
}

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

func (i *RawIterator) SeekToLast() {
	for _, it := range i.structure.Iterators() {
		it.SeekToLast()
	}
	reverseCmp := reverseComparator(i.structure.Cmp())
	tempStructure := data_structures.NewMergeStructure(data_structures.Heap, i.structure.Iterators(), reverseCmp)
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

func (i *RawIterator) Prev() {
	if !i.valid {
		return
	}
	reverseCmp := reverseComparator(i.structure.Cmp())
	tempStructure := data_structures.NewMergeStructure(data_structures.Heap, i.structure.Iterators(), reverseCmp)
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

func (i *RawIterator) Key() MemtableEntry {
	return *i.current
}

func (i *RawIterator) Value() MemtableEntry {
	return *i.current
}

type MergedMemtableIterator struct {
	rawIterator *RawIterator
	current     *MemtableEntry
	prevKey     []byte
	valid       bool
}

func NewMergedMemtableIterator(rawIterator *RawIterator) include.Iterator[MemtableEntry] {
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

func (m *MergedMemtableIterator) SeekToFirst() {
	if m.rawIterator.structure == nil {
		m.valid = false
		return
	}
	m.prevKey = nil
	m.rawIterator.SeekToFirst()
	m.advance()
}

func (m *MergedMemtableIterator) SeekToLast() {
	m.prevKey = nil
	m.rawIterator.SeekToLast()
	m.advance()
}

func (m *MergedMemtableIterator) Seek(key MemtableEntry) {
	m.prevKey = nil
	m.rawIterator.Seek(key)
	m.advance()
}

func (m *MergedMemtableIterator) Next() {
	if !m.valid {
		return
	}
	m.prevKey = m.current.Key
	m.rawIterator.Next()
	m.advance()
}

func (m *MergedMemtableIterator) Prev() {
	panic("Prev not implemented")
}

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
