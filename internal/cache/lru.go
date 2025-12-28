package cache

import "container/list"

type LRU struct {
	maxElements int
	list        *list.List
	cache       map[interface{}]*list.Element
}

// entry korisnti bilo sta za key/value ako treba moze lako da se promeni
type entry struct {
	key   interface{}
	value interface{}
}

func NewLRU(maxElements int) *LRU {
	return &LRU{
		maxElements: maxElements,
		list:        list.New(),
		cache:       make(map[interface{}]*list.Element),
	}
}

func (lru *LRU) Get(key interface{}) (interface{}, bool) {
	node, ok := lru.cache[key]
	if !ok {
		return nil, false
	}

	lru.list.MoveToFront(node)

	return node.Value.(*entry).value, true
}

func (lru *LRU) Put(key interface{}, value interface{}) {
	if e, ok := lru.cache[key]; ok {
		e.Value.(*entry).value = value
		lru.list.MoveToFront(e)
		return
	}

	elem := lru.list.PushFront(&entry{key, value})
	lru.cache[key] = elem

	if lru.list.Len() > lru.maxElements {
		lru.removeLast()
	}
}

func (lru *LRU) removeLast() {
	element := lru.list.Back()
	if element == nil {
		return
	}

	lru.list.Remove(element)
	key := element.Value.(*entry).key
	delete(lru.cache, key)
}
