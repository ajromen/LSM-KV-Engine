package data_structures

import "github.com/ajromen/LSM-KV-Engine/internal/include"

type PriorityQueueItem[T any] struct {
	iterator include.Iterator[T]
}

type HeapPriorityQueue[T any] struct {
	data []*PriorityQueueItem[T]
	cmp  Comparator[T]
}

func (h *HeapPriorityQueue[T]) isEmpty() bool {
	return len(h.data) == 0
}

func (h *HeapPriorityQueue[T]) parent(j int) int {
	return (j - 1) / 2
}

func (h *HeapPriorityQueue[T]) leftChild(j int) int {
	return 2*j + 1
}

func (h *HeapPriorityQueue[T]) rightChild(j int) int {
	return 2*j + 2
}

func (h *HeapPriorityQueue[T]) hasLeft(j int) bool {
	return h.leftChild(j) < len(h.data)
}

func (h *HeapPriorityQueue[T]) hasRight(j int) bool {
	return h.rightChild(j) < len(h.data)
}

func (h *HeapPriorityQueue[T]) swap(i, j int) {
	h.data[i], h.data[j] = h.data[j], h.data[i]
}

func (h *HeapPriorityQueue[T]) upheap(j int) {
	parent := h.parent(j)
	if j > 0 && h.cmp(
		h.data[j].iterator.Key(),
		h.data[parent].iterator.Key(),
	) < 0 {
		h.swap(j, parent)
		h.upheap(parent)
	}
}

func (h *HeapPriorityQueue[T]) downheap(j int) {
	if h.hasLeft(j) {
		left := h.leftChild(j)
		smallChild := left
		if h.hasRight(j) {
			right := h.rightChild(j)
			if h.cmp(
				h.data[right].iterator.Key(),
				h.data[left].iterator.Key(),
			) < 0 {

				smallChild = right
			}
		}
		if h.cmp(
			h.data[smallChild].iterator.Key(),
			h.data[j].iterator.Key(),
		) < 0 {
			h.swap(j, smallChild)
			h.downheap(smallChild)
		}
	}
}

func (h *HeapPriorityQueue[T]) heapify() {
	n := len(h.data)
	for i := n/2 - 1; i >= 0; i-- {
		h.downheap(i)
	}
}

func NewHeapPriorityQueue[T any](iters []include.Iterator[T], cmp Comparator[T]) *HeapPriorityQueue[T] {
	h := &HeapPriorityQueue[T]{
		data: make([]*PriorityQueueItem[T], 0, len(iters)),
		cmp:  cmp,
	}
	for _, it := range iters {
		if it != nil && it.Valid() {
			h.data = append(h.data, &PriorityQueueItem[T]{iterator: it})
		}
	}
	h.heapify()
	return h
}

func (h *HeapPriorityQueue[T]) Winner() include.Iterator[T] {
	if h.isEmpty() {
		return nil
	}
	return h.data[0].iterator
}

func (h *HeapPriorityQueue[T]) Update(it include.Iterator[T]) {
	for idx, item := range h.data {
		if item.iterator == it {
			if !it.Valid() {
				last := len(h.data) - 1
				h.data[idx] = h.data[last]
				h.data = h.data[:last]
				if idx < len(h.data) {
					h.downheap(idx)
				}
			} else {
				h.downheap(idx)
				h.upheap(idx)
			}
			return
		}
	}
}

func (h *HeapPriorityQueue[T]) Advance(it include.Iterator[T]) {
	if h.isEmpty() {
		return
	}
	it.Next()
	if it.Valid() {
		h.data[0].iterator = it
		h.downheap(0)
		return
	}
	last := len(h.data) - 1
	h.data[0] = h.data[last]
	h.data = h.data[:last]
	if len(h.data) > 0 {
		h.downheap(0)
	}
}

func (h *HeapPriorityQueue[T]) Iterators() []include.Iterator[T] {
	iterators := []include.Iterator[T]{}
	for _, it := range h.data {
		iterators = append(iterators, it.iterator)
	}
	return iterators
}

func (h *HeapPriorityQueue[T]) Cmp() Comparator[T] {
	return h.cmp
}
