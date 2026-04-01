package structures

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
)

type Comparator[T any] func(a, b T) int
type SkipListNode[T any] struct {
	entry T                  // entry in a node
	next  []*SkipListNode[T] // pointers to next node across all levels
}
type SkipList[T any] struct {
	header   *SkipListNode[T]
	level    int
	maxLevel int
	cmp      Comparator[T]
	size     int
}

type SkipListIterator[T any] struct {
	list    *SkipList[T]
	current *SkipListNode[T]
}

func NewSkipListNode[T any](entry T, level int) *SkipListNode[T] {
	return &SkipListNode[T]{
		entry: entry,
		next:  make([]*SkipListNode[T], level),
	}
}

func NewSkipList[T any](maxLevel int, cmp Comparator[T]) *SkipList[T] {
	var zero T
	headerEntry := zero
	return &SkipList[T]{
		header:   NewSkipListNode[T](headerEntry, maxLevel),
		level:    1,
		maxLevel: maxLevel,
		cmp:      cmp,
	}
}

func (sl *SkipList[T]) randLevel() int {
	level := 1
	for rand.Float32() < 0.5 && level < sl.maxLevel {
		level++
	}
	return level
}

func (sl *SkipList[T]) Insert(entry T) {
	update := make([]*SkipListNode[T], sl.maxLevel)
	current := sl.header
	for i := sl.level - 1; i >= 0; i-- {
		for current.next[i] != nil && sl.cmp(current.next[i].entry, entry) < 0 {
			current = current.next[i]
		}
		update[i] = current
	}
	current = current.next[0]
	if current != nil && sl.cmp(current.entry, entry) == 0 {
		current.entry = entry
	} else {
		randLevel := sl.randLevel()
		if randLevel > sl.level {
			for i := sl.level; i < randLevel; i++ {
				update[i] = sl.header
			}
			sl.level = randLevel
		}
		newNode := NewSkipListNode[T](entry, randLevel)
		for i := 0; i < randLevel; i++ {
			newNode.next[i] = update[i].next[i]
			update[i].next[i] = newNode
		}
		sl.size++
	}
}

func (sl *SkipList[T]) Delete(entry T) error {
	update := make([]*SkipListNode[T], sl.maxLevel)
	current := sl.header
	for i := sl.level - 1; i >= 0; i-- {
		for current.next[i] != nil && sl.cmp(current.next[i].entry, entry) < 0 {
			current = current.next[i]
		}
		update[i] = current
	}
	target := current.next[0]
	if target == nil || sl.cmp(target.entry, entry) != 0 {
		return errors.New("element not found")
	}
	for i := 0; i < sl.level; i++ {
		if update[i].next[i] != target {
			break
		}
		update[i].next[i] = target.next[i]
	}
	for sl.level > 1 && sl.header.next[sl.level-1] == nil {
		sl.level--
	}
	sl.size--
	return nil
}

func (sl *SkipList[T]) MarkDeleted(entry T, setDeleted func(*T)) {
	setDeleted(&entry)
	sl.Insert(entry)
}

func (sl *SkipList[T]) LowerBound(entry T) (T, bool) {
	current := sl.header
	for i := sl.level - 1; i >= 0; i-- {
		for current.next[i] != nil &&
			sl.cmp(current.next[i].entry, entry) < 0 {
			current = current.next[i]
		}
	}
	current = current.next[0]
	if current != nil {
		return current.entry, true
	}
	var zero T
	return zero, false
}

func (sl *SkipList[T]) UpperBound(entry T) (T, bool) {
	current := sl.header
	for i := sl.level - 1; i >= 0; i-- {
		for current.next[i] != nil &&
			sl.cmp(current.next[i].entry, entry) <= 0 {
			current = current.next[i]
		}
	}
	current = current.next[0]
	if current != nil {
		return current.entry, true
	}
	var zero T
	return zero, false
}

func (sl *SkipList[T]) Search(entry T) (T, bool) {
	current := sl.header
	for i := sl.level - 1; i >= 0; i-- {
		for current.next[i] != nil && sl.cmp(current.next[i].entry, entry) < 0 {
			current = current.next[i]
		}
	}
	current = current.next[0]
	if current != nil && sl.cmp(current.entry, entry) == 0 {
		return current.entry, true
	}
	var zero T
	return zero, false
}

func (sl *SkipList[T]) EntriesInOrder() []T {
	it := sl.Iterator()
	it.SeekToFirst()
	result := make([]T, 0, sl.size)
	for it.Valid() {
		result = append(result, it.Value())
		it.Next()
	}
	return result
}

func (sl *SkipList[T]) Reset() {
	var zero T
	sl.header = NewSkipListNode[T](zero, sl.maxLevel)
	sl.level = 1
	sl.size = 0
}

func (sl *SkipList[T]) Size() int {
	return sl.size
}

func (sl *SkipList[T]) Iterator() *SkipListIterator[T] {
	return &SkipListIterator[T]{
		list:    sl,
		current: nil,
	}
}

func (it *SkipListIterator[T]) Valid() bool {
	return it.current != nil
}

func (it *SkipListIterator[T]) SeekToFirst() {
	it.current = it.list.header.next[0]
}

func (it *SkipListIterator[T]) SeekToLast() {
	current := it.list.header

	for i := it.list.level - 1; i >= 0; i-- {
		for current.next[i] != nil {
			current = current.next[i]
		}
	}

	if current == it.list.header {
		it.current = nil
	} else {
		it.current = current
	}
}

func (it *SkipListIterator[T]) Seek(key T) {
	current := it.list.header

	for i := it.list.level - 1; i >= 0; i-- {
		for current.next[i] != nil &&
			it.list.cmp(current.next[i].entry, key) < 0 {
			current = current.next[i]
		}
	}

	it.current = current.next[0]
}

func (it *SkipListIterator[T]) Next() {
	if it.current != nil {
		it.current = it.current.next[0]
	}
}

func (it *SkipListIterator[T]) Prev() {

	if it.current == nil {
		return
	}

	current := it.list.header
	var prev *SkipListNode[T]

	for i := it.list.level - 1; i >= 0; i-- {
		for current.next[i] != nil &&
			it.list.cmp(current.next[i].entry, it.current.entry) < 0 {
			prev = current.next[i]
			current = current.next[i]
		}
	}

	it.current = prev
}

func (it *SkipListIterator[T]) Key() T {
	if it.current == nil {
		var zero T
		return zero
	}
	return it.current.entry
}

func (it *SkipListIterator[T]) Value() T {
	if it.current == nil {
		var zero T
		return zero
	}
	return it.current.entry
}

func (sl *SkipList[T]) Visualize(formatter func(T) string) string {
	if sl.header == nil {
		return "Empty skip list\n"
	}

	var result strings.Builder

	baseNodes := []*SkipListNode[T]{}
	current := sl.header.next[0]
	for current != nil {
		baseNodes = append(baseNodes, current)
		current = current.next[0]
	}

	if len(baseNodes) == 0 {
		return "SkipList is empty\n"
	}

	indexMap := make(map[*SkipListNode[T]]int)
	for i, node := range baseNodes {
		indexMap[node] = i
	}

	result.WriteString("\n========== SKIP LIST ==========\n")

	for level := sl.level - 1; level >= 0; level-- {
		line := make([]string, len(baseNodes))

		for i := range line {
			line[i] = "        "
		}

		current := sl.header.next[level]
		for current != nil {
			idx := indexMap[current]
			line[idx] = fmt.Sprintf("%-8s", formatter(current.entry))
			current = current.next[level]
		}

		result.WriteString(fmt.Sprintf("Level %d: %s\n", level, strings.Join(line, " -> ")))
	}

	result.WriteString("================================\n")
	return result.String()
}
