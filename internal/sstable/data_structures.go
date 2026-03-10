package sstable

import (
	"bytes"
	"math"

	"github.com/ajromen/LSM-KV-Engine/internal/utils"
)

type MergeStructure interface {
	Winner() *DataBlockIterator
	Update(it *DataBlockIterator)
}

/*
Winner Tree (a.k.a. tournament tree) for merging multiple SSTable iterators.
-each leaf represents an iterator of a single SSTable.
-internal nodes store the "winner" between their two children.
-the root contains the global winner (the smallest key).
	              [Winner]
	             /        \
	          [W01]      [W23]
	         /    \     /    \
	       I0      I1  I2      I3
When a leaf iterator advances (Next), all nodes on the path to the root are updated to find the new winner.
Advantages:
-quickly find the current smallest element: O(1) via Winner()
-update after advancing an iterator: O(log k), where k = number of leaves
-efficient merge of multiple sorted sequences
*/

type WinnerTreeNode struct {
	Iterator *DataBlockIterator // iterator for each sstable
}

type WinnerTree struct {
	nodes      []WinnerTreeNode           // all nodes of tree represented as array
	k          int                        // number of leaves - number of sstable iterators provided
	leafIndex  map[*DataBlockIterator]int // quickly find the winner among leaves
	pathToRoot [][]int                    // path of node indexes to root for each leaf
}

func NewWinnerTree(iters []*DataBlockIterator) *WinnerTree {
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
			nodes[i] = WinnerTreeNode{Iterator: iters[i]}
		} else {
			nodes[i] = WinnerTreeNode{Iterator: nil}
		}
	}
	leafIndex := make(map[*DataBlockIterator]int, len(iters))
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

func compare(a, b *DataBlockIterator) *DataBlockIterator {
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
		wt.nodes[n] = WinnerTreeNode{Iterator: compare(wt.nodes[left].Iterator, wt.nodes[right].Iterator)}
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

func (wt *WinnerTree) Winner() *DataBlockIterator {
	root := 2*wt.k - 2
	return wt.nodes[root].Iterator
}

func (wt *WinnerTree) Update(it *DataBlockIterator) {
	leafIdx, ok := wt.leafIndex[it]
	if !ok {
		return
	}
	it.Next()
	wt.nodes[leafIdx] = WinnerTreeNode{Iterator: it}
	cur := leafIdx
	for _, internalIdx := range wt.pathToRoot[leafIdx] {
		left := 2 * (internalIdx - wt.k)
		right := left + 1
		wt.nodes[internalIdx].Iterator = compare(wt.nodes[left].Iterator, wt.nodes[right].Iterator)
		cur = internalIdx
		_ = cur
	}
}

/*
Min-Heap (Priority Queue) for merging multiple SSTable iterators.
-each element in the heap represents a currently valid iterator.
-the root of the heap always contains the global winner (the smallest key).
-when the root iterator advances (Next), it is removed from the root, updated, and re-inserted into the heap to maintain the min-heap property.

Example with 4 iterators (I0, I1, I2, I3) in heap order:

          [Winner (root)]
          /       \
       [I1]       [I2]
      /    \
    [I0]    [I3]

-root = iterator with smallest key
-when root advances, heap is adjusted using upheap/downheap to restore the property

Advantages:
-quickly find the current smallest element: O(1) via root
-update after advancing an iterator: O(log k), where k = number of iterators
-simple and flexible structure for merging multiple sorted sequences
*/

type PriorityQueueItem struct {
	iterator *DataBlockIterator
}

type HeapPriorityQueue struct {
	data []*PriorityQueueItem
}

func (h *HeapPriorityQueue) isEmpty() bool {
	return len(h.data) == 0
}

func (h *HeapPriorityQueue) parent(j int) int {
	return (j - 1) / 2
}

func (h *HeapPriorityQueue) leftChild(j int) int {
	return 2*j + 1
}

func (h *HeapPriorityQueue) rightChild(j int) int {
	return 2*j + 2
}

func (h *HeapPriorityQueue) hasLeft(j int) bool {
	return h.leftChild(j) < len(h.data)
}

func (h *HeapPriorityQueue) hasRight(j int) bool {
	return h.rightChild(j) < len(h.data)
}

func (h *HeapPriorityQueue) swap(i, j int) {
	tmp := h.data[i]
	h.data[i] = h.data[j]
	h.data[j] = tmp
}

func (h *HeapPriorityQueue) upheap(j int) {
	parent := h.parent(j)
	if j > 0 && compare(h.data[j].iterator, h.data[parent].iterator) == h.data[j].iterator {
		h.swap(j, parent)
		h.upheap(parent)
	}
}

func (h *HeapPriorityQueue) downheap(j int) {
	if h.hasLeft(j) {
		left := h.leftChild(j)
		smallChild := left
		if h.hasRight(j) {
			right := h.rightChild(j)
			if compare(h.data[left].iterator, h.data[right].iterator) == h.data[right].iterator {
				smallChild = right
			}
		}
		if compare(h.data[smallChild].iterator, h.data[j].iterator) == h.data[smallChild].iterator {
			h.swap(j, smallChild)
			h.downheap(smallChild)
		}
	}
}

func NewHeapPriorityQueue(iters []*DataBlockIterator) *HeapPriorityQueue {
	h := &HeapPriorityQueue{
		data: make([]*PriorityQueueItem, 0, len(iters)),
	}
	for _, it := range iters {
		if it != nil && it.Valid() {
			h.data = append(h.data, &PriorityQueueItem{iterator: it})
		}
	}
	h.heapify()
	return h
}

func (h *HeapPriorityQueue) heapify() {
	n := len(h.data)
	for i := n/2 - 1; i >= 0; i-- {
		h.downheap(i)
	}
}

func (h *HeapPriorityQueue) Winner() *DataBlockIterator {
	if h.isEmpty() {
		return nil
	}
	return h.data[0].iterator
}

func (h *HeapPriorityQueue) Update(it *DataBlockIterator) {
	if h.isEmpty() {
		return
	}
	last := len(h.data) - 1
	h.data[0] = h.data[last]
	h.data = h.data[:last]
	it.Next()
	if it.Valid() {
		h.data = append(h.data, &PriorityQueueItem{iterator: it})
		h.upheap(len(h.data) - 1)
	}
	if len(h.data) > 0 {
		h.downheap(0)
	}
}

//

const (
	Heap  byte = 0
	WTree byte = 1
)

func NewMergeStructure(t byte, iters []*DataBlockIterator) MergeStructure {
	switch t {
	case Heap:
		return NewHeapPriorityQueue(iters)
	case WTree:
		return NewWinnerTree(iters)
	default:
		return NewWinnerTree(iters)
	}
}
