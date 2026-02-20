package data_structures

import "math/rand"

type Node[T any] struct {
	entry T
	next  []*Node[T]
}

type SkipList[T any] struct {
	header   *Node[T]
	level    int
	size     int
	MaxLevel int
	cmp      Comparator[T]
}

func NewNode[T any](entry T, level int) *Node[T] {
	return &Node[T]{
		entry: entry,
		next:  make([]*Node[T], level),
	}
}
func NewSkipList[T any](cmp Comparator[T], MaxLevel int) *SkipList[T] {
	var zero T
	headerEntry := zero
	return &SkipList[T]{
		header:   NewNode[T](headerEntry, MaxLevel),
		level:    1,
		size:     0,
		MaxLevel: MaxLevel,
		cmp:      cmp,
	}
}

func (s *SkipList[T]) randLevel() int {
	level := 1
	for rand.Float32() < 0.5 && level < s.MaxLevel {
		level++
	}
	return level
}

func (s *SkipList[T]) Insert(entry T) {
	update := make([]*Node[T], s.MaxLevel)
	current := s.header
	for i := s.level - 1; i >= 0; i-- {
		for current.next[i] != nil && s.cmp(current.next[i].entry, entry) < 0 {
			current = current.next[i]
		}
		update[i] = current
	}
	current = current.next[0]
	if current != nil && s.cmp(current.entry, entry) == 0 {
		current.entry = entry
	} else {
		randLevel := s.randLevel()
		if randLevel > s.level {
			for i := s.level; i < randLevel; i++ {
				update[i] = s.header
			}
			s.level = randLevel
		}
		newNode := NewNode[T](entry, randLevel)
		for i := 0; i < randLevel; i++ {
			newNode.next[i] = update[i].next[i]
			update[i].next[i] = newNode
		}
		s.size++
	}
}

func (s *SkipList[T]) MarkDeleted(entry T, setDeleted func(*T)) {
	setDeleted(&entry)
	s.Insert(entry)
}

func (s *SkipList[T]) Search(entry T) (T, bool) {
	current := s.header
	for i := s.level - 1; i >= 0; i-- {
		for current.next[i] != nil && s.cmp(current.next[i].entry, entry) < 0 {
			current = current.next[i]
		}
	}
	current = current.next[0]
	if current != nil && s.cmp(current.entry, entry) == 0 {
		return current.entry, true
	}
	var zero T
	return zero, false
}

func (s *SkipList[T]) EntriesInOrder() []T {
	entries := make([]T, 0, s.size)
	current := s.header.next[0]
	for current != nil {
		entries = append(entries, current.entry)
		current = current.next[0]
	}
	return entries
}

func (s *SkipList[T]) Reset() {
	var zero T
	s.header = NewNode[T](zero, s.MaxLevel)
	s.level = 1
	s.size = 0
}

func (s *SkipList[T]) Size() int {
	return s.size
}
