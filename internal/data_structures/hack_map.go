package data_structures

import (
	"fmt"
	"sort"
	"strings"
)

type HasKey interface {
	HashKey() string
}
type HackMapNode[T HasKey] struct {
	entry T
}

type HackMap[T HasKey] struct {
	data map[string]*Stack[T]
}

type HackMapIterator[T HasKey] struct {
	hackmap  *HackMap[T]
	keys     []string
	keyIndex int
	valIndex int
	current  *HackMapNode[T]
}

func NewHackMap[T HasKey]() *HackMap[T] {
	return &HackMap[T]{
		data: make(map[string]*Stack[T]),
	}
}

func (hm *HackMap[T]) Insert(entry T) {
	key := entry.HashKey()
	stack, ok := hm.data[key]
	if !ok {
		stack = New[T]()
		hm.data[key] = stack
	}
	stack.Push(entry)
}

func (hm *HackMap[T]) LowerBound(target string) (T, bool) {
	keys := hm.KeysInOrder()
	low, high := 0, len(keys)-1
	var result string
	for low <= high {
		mid := (low + high) / 2
		if keys[mid] >= target {
			result = keys[mid]
			high = mid - 1
		} else {
			low = mid + 1
		}
	}
	if result == "" {
		var zero T
		return zero, false
	}
	stack := hm.data[result]
	return stack.Peek(), true
}

func (hm *HackMap[T]) UpperBound(target string) (T, bool) {
	keys := hm.KeysInOrder()
	low, high := 0, len(keys)-1
	var result string
	for low <= high {
		mid := (low + high) / 2
		if keys[mid] > target {
			result = keys[mid]
			high = mid - 1
		} else {
			low = mid + 1
		}
	}
	if result == "" {
		var zero T
		return zero, false
	}
	stack := hm.data[result]
	return stack.Peek(), true
}

func (hm *HackMap[T]) Search(entry T) *T {
	stack, ok := hm.data[entry.HashKey()]
	if !ok || stack.Len() == 0 {
		return nil
	}
	result := stack.Peek()
	return &result
}

func (hm *HackMap[T]) Remove(entry T) {
	key := entry.HashKey()
	stack, ok := hm.data[key]
	if !ok {
		return
	}
	stack.Pop()
	delete(hm.data, key)
}

func (hm *HackMap[T]) KeysInOrder() []string {
	keys := make([]string, 0)
	for k := range hm.data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (hm *HackMap[T]) EntriesInOrder() []T {
	it := hm.Iterator()
	it.SeekToFirst()
	var result []T
	for it.Valid() {
		result = append(result, it.Value())
		it.Next()
	}
	return result
}

func (hm *HackMap[T]) Data() map[string]*Stack[T] {
	return hm.data
}

func (hm *HackMap[T]) Size() int {
	return len(hm.data)
}

func (hm *HackMap[T]) Reset() {
	hm.data = make(map[string]*Stack[T])
}

func (hm *HackMap[T]) Iterator() *HackMapIterator[T] {
	return &HackMapIterator[T]{
		hackmap: hm,
		current: nil,
	}
}

func (it *HackMapIterator[T]) Valid() bool {
	if it.keyIndex < 0 || it.keyIndex >= len(it.keys) {
		return false
	}
	stack := it.hackmap.data[it.keys[it.keyIndex]]
	return it.valIndex >= 0 && it.valIndex < stack.Len()
}

func (it *HackMapIterator[T]) SeekToFirst() {
	it.keys = it.hackmap.KeysInOrder()
	if len(it.keys) == 0 {
		it.keyIndex = -1
		return
	}
	it.keyIndex = 0
	stack := it.hackmap.data[it.keys[0]]
	it.valIndex = stack.Len() - 1
}

func (it *HackMapIterator[T]) SeekToLast() {
	it.keys = it.hackmap.KeysInOrder()
	if len(it.keys) == 0 {
		it.keyIndex = -1
		return
	}
	it.keyIndex = len(it.keys) - 1
	stack := it.hackmap.data[it.keys[len(it.keys)-1]]
	it.valIndex = stack.Len() - stack.Len()
}

func (it *HackMapIterator[T]) Seek(key T) {
	k := key.HashKey()
	it.keys = it.hackmap.KeysInOrder()
	it.keyIndex = -1
	for i, keyStr := range it.keys {
		if keyStr >= k {
			it.keyIndex = i
			break
		}
	}
	if it.keyIndex == -1 {
		it.valIndex = -1
		it.current = nil
		return
	}
	stack := it.hackmap.data[it.keys[it.keyIndex]]
	it.valIndex = stack.Len() - 1
	it.current = &HackMapNode[T]{entry: stack.Data()[it.valIndex]}
}

func (it *HackMapIterator[T]) Next() {
	if !it.Valid() {
		return
	}
	stack := it.hackmap.data[it.keys[it.keyIndex]]
	if it.valIndex > 0 {
		it.valIndex--
		return
	}
	it.keyIndex++
	if it.keyIndex >= len(it.keys) {
		it.keyIndex = -1
		return
	}
	stack = it.hackmap.data[it.keys[it.keyIndex]]
	it.valIndex = stack.Len() - 1
}

func (it *HackMapIterator[T]) Prev() {
	if it.keyIndex == -1 || len(it.keys) == 0 {
		it.SeekToLast()
		return
	}
	stack := it.hackmap.data[it.keys[it.keyIndex]]
	if it.valIndex < stack.Len()-1 {
		it.valIndex++
		return
	}
	it.keyIndex--
	if it.keyIndex < 0 {
		it.keyIndex = -1
		return
	}
	stack = it.hackmap.data[it.keys[it.keyIndex]]
	it.valIndex = 0
}

func (it *HackMapIterator[T]) Value() T {
	stack := it.hackmap.data[it.keys[it.keyIndex]]
	return stack.Data()[it.valIndex]
}

func (it *HackMapIterator[T]) Key() T {
	return it.Value()
}

func (hm *HackMap[T]) Visualize(formatter func(T) string) string {
	data := hm.Data()
	if len(data) == 0 {
		return "Memtable empty.\n"
	}

	maxKeyLen := len("Key")
	maxValLen := len("Value")

	type row struct {
		Key   string
		Value string
	}
	rows := make([]row, 0)

	for key, stack := range data {
		tmp := stack.Clone()
		for tmp.Len() > 0 {
			e := tmp.Pop()
			valStr := formatter(e)
			rows = append(rows, row{key, valStr})

			if len(key) > maxKeyLen {
				maxKeyLen = len(key)
			}
			if len(valStr) > maxValLen {
				maxValLen = len(valStr)
			}
		}
	}
	var sb strings.Builder
	sb.WriteString("====== HackMap Memtable ======\n")
	sb.WriteString(fmt.Sprintf("%-*s | %-*s\n", maxKeyLen, "Key", maxValLen, "Value"))
	sb.WriteString(strings.Repeat("-", maxKeyLen+maxValLen+3) + "\n")
	for _, r := range rows {
		sb.WriteString(fmt.Sprintf("%-*s | %-*s\n", maxKeyLen, r.Key, maxValLen, r.Value))
	}
	sb.WriteString("=============================\n")

	return sb.String()
}
