package data_structures

import "github.com/ajromen/LSM-KV-Engine/internal/include"

type MergeStructure[T any] interface {
	Winner() include.Iterator[T]
	Update(it include.Iterator[T])
	Advance(it include.Iterator[T])
	Iterators() []include.Iterator[T]
	Cmp() Comparator[T]
}

const (
	Heap  byte = 0
	WTree byte = 1
)

func NewMergeStructure[T any](t byte, iters []include.Iterator[T], cmp Comparator[T]) MergeStructure[T] {
	switch t {
	case Heap:
		return NewHeapPriorityQueue(iters, cmp)
	case WTree:
		return NewWinnerTree(iters, cmp)
	default:
		return NewWinnerTree(iters, cmp)
	}
}
