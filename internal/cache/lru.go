package cache

import "container/list"

type LRU[K comparable, V any] struct {
	maxElements int
	list        *list.List
	cache       map[K]*list.Element
}

// entry korisnti bilo sta za key/value ako treba moze lako da se promeni
type entry[K comparable, V any] struct {
	key   K
	value V
}

func NewLRU[K comparable, V any](maxElements int) *LRU[K, V] {
	return &LRU[K, V]{
		maxElements: maxElements,
		list:        list.New(),
		cache:       make(map[K]*list.Element),
	}
}

func (lru *LRU[K, V]) Get(key K) (V, bool) {
	node, ok := lru.cache[key]
	if !ok {
		var zero V
		return zero, false
	}

	lru.list.MoveToFront(node)

	return node.Value.(*entry[K, V]).value, true
}

func (lru *LRU[K, V]) Put(key K, value V) {
	if e, ok := lru.cache[key]; ok {
		e.Value.(*entry[K, V]).value = value
		lru.list.MoveToFront(e)
		return
	}

	elem := lru.list.PushFront(&entry[K, V]{key, value})
	lru.cache[key] = elem

	if lru.list.Len() > lru.maxElements {
		lru.removeLast()
	}
}

func (lru *LRU[K, V]) removeLast() {
	element := lru.list.Back()
	if element == nil {
		return
	}

	lru.list.Remove(element)
	key := element.Value.(*entry[K, V]).key
	delete(lru.cache, key)
}
