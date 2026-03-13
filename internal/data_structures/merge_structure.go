package data_structures

import "github.com/ajromen/LSM-KV-Engine/internal/iterator"

type MergeStructure[T any] interface {
	Winner() iterator.Iterator[T]
	Update(it iterator.Iterator[T])
	Advance(it iterator.Iterator[T])
	Iterators() []iterator.Iterator[T]
	Cmp() Comparator[T]
}

const (
	Heap  byte = 0
	WTree byte = 1
)

func NewMergeStructure[T any](t byte, iters []iterator.Iterator[T], cmp Comparator[T]) MergeStructure[T] {
	switch t {
	case Heap:
		return NewHeapPriorityQueue(iters, cmp)
	case WTree:
		return NewWinnerTree(iters, cmp)
	default:
		return NewWinnerTree(iters, cmp)
	}
}
