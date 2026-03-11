package data_structures

import (
	"math"

	"github.com/ajromen/LSM-KV-Engine/internal/include"
)

type WinnerTreeNode[T any] struct {
	Iterator include.Iterator[T]
}

type WinnerTree[T any] struct {
	nodes      []WinnerTreeNode[T]
	k          int
	leafIndex  map[include.Iterator[T]]int
	pathToRoot [][]int
	cmp        Comparator[T]
}

func NewWinnerTree[T any](iters []include.Iterator[T], cmp Comparator[T]) *WinnerTree[T] {
	if len(iters) == 0 {
		return nil
	}
	k := int(math.Pow(2, math.Ceil(math.Log2(float64(len(iters))))))
	if len(iters) == 1 {
		k = 1
	}
	nodes := make([]WinnerTreeNode[T], 2*k-1)
	for i := 0; i < k; i++ {
		if i < len(iters) {
			nodes[i] = WinnerTreeNode[T]{Iterator: iters[i]}
		}
	}
	leafIndex := make(map[include.Iterator[T]]int, len(iters))
	for i, it := range iters {
		leafIndex[it] = i
	}
	wt := &WinnerTree[T]{
		nodes:      nodes,
		k:          k,
		leafIndex:  leafIndex,
		pathToRoot: make([][]int, k),
		cmp:        cmp,
	}
	wt.build()
	wt.computePaths()
	return wt
}

func (wt *WinnerTree[T]) parentOf(c int) int {
	return wt.k + c/2
}

func (wt *WinnerTree[T]) build() {
	for n := wt.k; n < 2*wt.k-1; n++ {
		left := 2 * (n - wt.k)
		right := left + 1
		wt.nodes[n] = WinnerTreeNode[T]{
			Iterator: include.Compare(wt.nodes[left].Iterator, wt.nodes[right].Iterator, wt.cmp),
		}
	}
}

func (wt *WinnerTree[T]) computePaths() {
	root := 2*wt.k - 2
	for leaf := 0; leaf < wt.k; leaf++ {
		path := []int{}
		cur := leaf
		for cur != root {
			p := wt.parentOf(cur)
			path = append(path, p)
			cur = p
		}
		wt.pathToRoot[leaf] = path
	}
}

func (wt *WinnerTree[T]) Winner() include.Iterator[T] {
	root := 2*wt.k - 2
	return wt.nodes[root].Iterator
}

func (wt *WinnerTree[T]) Update(it include.Iterator[T]) {
	leafIdx, ok := wt.leafIndex[it]
	if !ok {
		return
	}
	wt.nodes[leafIdx] = WinnerTreeNode[T]{Iterator: it}
	for _, internalIdx := range wt.pathToRoot[leafIdx] {
		left := 2 * (internalIdx - wt.k)
		right := left + 1
		wt.nodes[internalIdx].Iterator = include.Compare(wt.nodes[left].Iterator, wt.nodes[right].Iterator, wt.cmp)
	}
}

func (wt *WinnerTree[T]) Advance(it include.Iterator[T]) {
	leafIdx, ok := wt.leafIndex[it]
	if !ok {
		return
	}
	it.Next()
	wt.nodes[leafIdx] = WinnerTreeNode[T]{Iterator: it}
	for _, internalIdx := range wt.pathToRoot[leafIdx] {
		left := 2 * (internalIdx - wt.k)
		right := left + 1
		wt.nodes[internalIdx].Iterator = include.Compare(wt.nodes[left].Iterator, wt.nodes[right].Iterator, wt.cmp)
	}
}

func (wt *WinnerTree[T]) Iterators() []include.Iterator[T] {
	iterators := make([]include.Iterator[T], 0, len(wt.leafIndex))
	for it := range wt.leafIndex {
		iterators = append(iterators, it)
	}
	return iterators
}

func (wt *WinnerTree[T]) Cmp() Comparator[T] {
	return wt.cmp
}
