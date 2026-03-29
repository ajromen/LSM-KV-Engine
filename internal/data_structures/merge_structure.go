package data_structures

import (
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
	"github.com/ajromen/LSM-KV-Engine/internal/iterator"
)

type MergeStructure[T any] interface {
	Winner() iterator.Iterator[T]
	Update(it iterator.Iterator[T])
	Advance(it iterator.Iterator[T])
	Iterators() []iterator.Iterator[T]
	Cmp() Comparator[T]
}

func NewMergeStructure[T any](t byte, iters []iterator.Iterator[T], cmp Comparator[T]) MergeStructure[T] {
	switch t {
	case byte(enums.Heap):
		return NewHeapPriorityQueue(iters, cmp)
	case byte(enums.WTree):
		return NewWinnerTree(iters, cmp)
	default:
		return NewWinnerTree(iters, cmp)
	}
}
