package data_structures

import (
	"bytes"
	"math"

	"github.com/ajromen/LSM-KV-Engine/internal/sstable"
	"github.com/ajromen/LSM-KV-Engine/internal/utils"
)

type WinnerTreeNode struct {
	iterator *sstable.DataBlockIterator // iterator for each sstable
}
type WinnerTree struct {
	nodes      []WinnerTreeNode                   // all nodes of tree represented as array
	k          int                                // number of leaves - number of sstable iterators provided
	leafIndex  map[*sstable.DataBlockIterator]int // quickly find the winner among leaves
	pathToRoot [][]int                            // path of node indexes to root for each leaf
}

func NewWinnerTree(iters []*sstable.DataBlockIterator) *WinnerTree {
	if len(iters) == 0 {
		return nil
	}
	k := int(math.Pow(2, math.Ceil(math.Log2(float64(len(iters))))))
	if len(iters) == 1 {
		k = 1
	}
	nodes := make([]WinnerTreeNode, 2*k-1)
	for i := 0; i < k; i++ {
		if i < len(iters) {
			nodes[i] = WinnerTreeNode{iterator: iters[i]}
		} else {
			nodes[i] = WinnerTreeNode{iterator: nil}
		}
	}
	leafIndex := make(map[*sstable.DataBlockIterator]int, len(iters))
	for i, it := range iters {
		leafIndex[it] = i
	}
	wt := &WinnerTree{
		nodes:      nodes,
		k:          k,
		leafIndex:  leafIndex,
		pathToRoot: make([][]int, k),
	}
	wt.build()
	wt.computePaths()
	return wt
}

func compare(a, b *sstable.DataBlockIterator) *sstable.DataBlockIterator {
	if a == nil || !a.Valid() {
		return b
	}
	if b == nil || !b.Valid() {
		return a
	}
	if bytes.Compare(a.Current().Key, b.Current().Key) < 0 {
		return a
	}
	if bytes.Compare(a.Current().Key, b.Current().Key) > 0 {
		return b
	}
	if utils.Uint128GE(a.Current().Timestamp, b.Current().Timestamp) {
		return a
	}
	return b
}

func (wt *WinnerTree) parentOf(c int) int {
	return wt.k + c/2
}

func (wt *WinnerTree) build() {
	for n := wt.k; n < 2*wt.k-1; n++ {
		left := 2 * (n - wt.k)
		right := left + 1
		wt.nodes[n] = WinnerTreeNode{iterator: compare(wt.nodes[left].iterator, wt.nodes[right].iterator)}
	}
}

func (wt *WinnerTree) computePaths() {
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

func (wt *WinnerTree) Winner() *sstable.DataBlockIterator {
	root := 2*wt.k - 2
	return wt.nodes[root].iterator
}

func (wt *WinnerTree) Update(it WinnerTreeNode) {
	leafIdx, ok := wt.leafIndex[it.iterator]
	if !ok {
		return
	}
	it.iterator.Next()
	wt.nodes[leafIdx] = it
	cur := leafIdx
	for _, internalIdx := range wt.pathToRoot[leafIdx] {
		left := 2 * (internalIdx - wt.k)
		right := left + 1
		wt.nodes[internalIdx].iterator = compare(wt.nodes[left].iterator, wt.nodes[right].iterator)
		cur = internalIdx
		_ = cur
	}
}
