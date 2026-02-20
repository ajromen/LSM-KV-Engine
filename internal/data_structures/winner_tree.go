package data_structures

import (
	"github.com/ajromen/LSM-KV-Engine/internal/sstable"
)

type WinnerTreeNode struct {
	iterator *sstable.DataBlockIterator // iterator for each sstable
}
type WinnerTree struct {
	nodes      []WinnerTreeNode                   // all nodes of tree represented as array
	k          int                                // number of leaves - number of sstable iterators provided
	root       int                                // index of root
	leafIndex  map[*sstable.DataBlockIterator]int // quickly find the winner among leaves
	pathToRoot [][]int                            // path of node indexes to root for each leaf
}

func NewWinnerTree(iters []*sstable.DataBlockIterator) *WinnerTree {
	k := len(iters)
	if k == 0 {
		return nil
	}
	nodes := make([]WinnerTreeNode, 2*k-1)
	leafIndex := make(map[*sstable.DataBlockIterator]int, k)
	for i := 0; i < k; i++ {
		nodes[i] = WinnerTreeNode{iterator: iters[i]}
		leafIndex[iters[i]] = i
	}
	wt := &WinnerTree{
		nodes:      nodes,
		k:          k,
		root:       2*k - 2,
		leafIndex:  leafIndex,
		pathToRoot: make([][]int, k),
	}
	//implement build and call it
	//implement path computing and call it
	return wt
}
