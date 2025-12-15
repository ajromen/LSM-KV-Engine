package memtable

import "math/rand"

type Node struct {
	key   string
	value []byte
	next  []*Node
}

type SkipList struct {
	header *Node
	level  int
	size   int
}

func NewNode(key string, value []byte, level int) *Node {
	return &Node{
		key:   key,
		value: value,
		next:  make([]*Node, level),
	}
}

func NewSkipList(MaxLevel int) *SkipList {
	return &SkipList{
		header: NewNode("", []byte{}, MaxLevel),
		level:  1,
		size:   0,
	}
}

func (sl *SkipList) findRandLevel(MaxLevel int) int {
	level := 1
	for rand.Float32() < 0.5 && level <= MaxLevel {
		level++
	}
	return level
}

func (sl *SkipList) Insert(key string, value []byte, MaxLevel int) {
	update := make([]*Node, MaxLevel)
	current := sl.header
	for i := sl.level - 1; i >= 0; i-- {
		for current.next[i] != nil && current.key < key {
			current = current.next[i]
		}
		update[i] = current
	}
	current = current.next[0]
	if current != nil && current.key == key {
		current.value = value
	} else {
		randLevel := sl.findRandLevel(MaxLevel)
		if randLevel > sl.level {
			for i := sl.level; i <= randLevel; i++ {
				update[i] = sl.header
			}
			sl.level = randLevel
		}
		newNode := NewNode(key, value, randLevel)
		for i := 0; i < randLevel; i++ {
			newNode.next[i] = update[i].next[i]
			update[i].next[i] = newNode
		}
		sl.size++
	}
}

func (sl *SkipList) Remove(key string, MaxLevel int) {
	update := make([]*Node, MaxLevel)
	current := sl.header
	for i := sl.level - 1; i >= 0; i-- {
		for current.next[i] != nil && current.key < key {
			current = current.next[i]
		}
		update[i] = current
	}
	current = current.next[0]
	if current != nil && current.key == key {
		for i := 0; i < sl.level; i++ {
			if update[i].next[i] != current {
				break
			}
			update[i].next[i] = current.next[i]
		}
		for sl.level > 1 && sl.header.next[sl.level] == nil {
			sl.level = sl.level - 1
		}
		sl.size--
	}
}

func (sl *SkipList) Search(key string, MaxLevel int) ([]byte, bool) {
	current := sl.header
	for i := sl.level - 1; i >= 0; i-- {
		for current.next[i] != nil && current.key < key {
			current = current.next[i]
		}
	}
	current = current.next[0]
	if current != nil && current.key == key {
		return current.value, true
	}
	return []byte{}, false
}

type SkipListMemtable struct {
	memtableData *SkipList
	maxSize      int
}

func NewSkipListMem(maxSize int) *SkipListMemtable {
	return &SkipListMemtable{
		memtableData: NewSkipList(16),
		maxSize:      maxSize,
	}
}

func (memtable *SkipListMemtable) Put(key string, value []byte) {
	memtable.memtableData.Insert(key, value, 16)
}

func (memtable *SkipListMemtable) Get(key string) ([]byte, bool) {
	return memtable.memtableData.Search(key, 16)
}

func (memtable *SkipListMemtable) Delete(key string) {
	memtable.memtableData.Remove(key, 16)
}

func (memtable *SkipListMemtable) Flush() bool {
	return memtable.memtableData.size >= memtable.maxSize
}
