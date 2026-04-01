package structures

import (
	"strings"
)

type BTreeNode[T any] struct {
	nodeData []T
	leaf     bool
	children []*BTreeNode[T]
}

type BTree[T any] struct {
	root *BTreeNode[T]
	t    int
	cmp  Comparator[T]
	size int
}

type stackEntry[T any] struct {
	node  *BTreeNode[T]
	index int
}
type BTreeIterator[T any] struct {
	tree           *BTree[T]
	currentNode    *BTreeNode[T]
	indexInNode    int
	stackOfParents Stack[stackEntry[T]]
}

func NewBTree[T any](t int, cmp Comparator[T]) *BTree[T] {
	nodeEntry := &BTreeNode[T]{
		nodeData: make([]T, 0),
		leaf:     true,
		children: make([]*BTreeNode[T], 0),
	}
	return &BTree[T]{
		root: nodeEntry,
		t:    t,
		cmp:  cmp,
		size: 0,
	}
}

func (btn *BTreeNode[T]) lowerBound(target T, cmp Comparator[T]) (T, bool) {
	i := 0
	for i < len(btn.nodeData) && cmp(btn.nodeData[i], target) < 0 {
		i++
	}
	if btn.leaf {
		if i < len(btn.nodeData) {
			return btn.nodeData[i], true
		}
		var zero T
		return zero, false
	}
	if i < len(btn.children) {
		if val, found := btn.children[i].lowerBound(target, cmp); found {
			return val, true
		}
	}
	if i < len(btn.nodeData) {
		return btn.nodeData[i], true
	}
	var zero T
	return zero, false
}

func (bt *BTree[T]) LowerBound(target T) (T, bool) {
	if bt.root == nil {
		var zero T
		return zero, false
	}
	return bt.root.lowerBound(target, bt.cmp)
}

func (btn *BTreeNode[T]) upperBound(target T, cmp Comparator[T]) (T, bool) {
	i := 0
	for i < len(btn.nodeData) && cmp(btn.nodeData[i], target) <= 0 {
		i++
	}
	if btn.leaf {
		if i < len(btn.nodeData) {
			return btn.nodeData[i], true
		}
		var zero T
		return zero, false
	}
	if i < len(btn.children) {
		if val, found := btn.children[i].upperBound(target, cmp); found {
			return val, true
		}
	}
	if i < len(btn.nodeData) {
		return btn.nodeData[i], true
	}
	var zero T
	return zero, false
}

func (bt *BTree[T]) UpperBound(target T) (T, bool) {
	if bt.root == nil {
		var zero T
		return zero, false
	}
	return bt.root.upperBound(target, bt.cmp)
}

func (btn *BTreeNode[T]) insertNonFull(val T, t int, cmp Comparator[T]) {
	i := len(btn.nodeData) - 1
	if btn.leaf {
		btn.nodeData = append(btn.nodeData, val)
		for i >= 0 && cmp(val, btn.nodeData[i]) < 0 {
			btn.nodeData[i+1] = btn.nodeData[i]
			i--
		}
		btn.nodeData[i+1] = val
		return
	}
	for i >= 0 && cmp(val, btn.nodeData[i]) < 0 {
		i--
	}
	i++
	if len(btn.children[i].nodeData) == 2*t-1 {
		btn.splitChild(i, t)
		if cmp(val, btn.nodeData[i]) > 0 {
			i++
		}
	}
	btn.children[i].insertNonFull(val, t, cmp)
}

func (btn *BTreeNode[T]) splitChild(i int, t int) {
	fullNode := btn.children[i]
	newNode := &BTreeNode[T]{
		nodeData: make([]T, t-1),
		leaf:     fullNode.leaf,
	}
	for j := 0; j < t-1; j++ {
		newNode.nodeData[j] = fullNode.nodeData[j+t]
	}
	if !fullNode.leaf {
		newNode.children = make([]*BTreeNode[T], t)
		for j := 0; j < t; j++ {
			newNode.children[j] = fullNode.children[j+t]
		}
		fullNode.children = fullNode.children[:t]
	}
	median := fullNode.nodeData[t-1]
	fullNode.nodeData = fullNode.nodeData[:t-1]
	btn.children = append(btn.children, nil)
	for j := len(btn.children) - 1; j > i+1; j-- {
		btn.children[j] = btn.children[j-1]
	}
	btn.children[i+1] = newNode
	btn.nodeData = append(btn.nodeData, *new(T))
	for j := len(btn.nodeData) - 1; j > i; j-- {
		btn.nodeData[j] = btn.nodeData[j-1]
	}
	btn.nodeData[i] = median
}

func (btn *BTreeNode[T]) inOrder(result *[]T) {
	if btn.leaf {
		*result = append(*result, btn.nodeData...)
		return
	}
	for i := 0; i < len(btn.nodeData); i++ {
		btn.children[i].inOrder(result)
		*result = append(*result, btn.nodeData[i])
	}
	btn.children[len(btn.nodeData)].inOrder(result)
}

func (bt *BTree[T]) Insert(val T) {
	root := bt.root
	if len(root.nodeData) == 2*bt.t-1 {
		s := &BTreeNode[T]{leaf: false, children: []*BTreeNode[T]{root}}
		bt.root = s
		s.splitChild(0, bt.t)
		s.insertNonFull(val, bt.t, bt.cmp)
	} else {
		root.insertNonFull(val, bt.t, bt.cmp)
	}
	bt.size++
}

func (btn *BTreeNode[T]) delete(key T, t int, cmp Comparator[T]) bool {
	i := 0
	for i < len(btn.nodeData) && cmp(key, btn.nodeData[i]) > 0 {
		i++
	}
	if i < len(btn.nodeData) && cmp(key, btn.nodeData[i]) == 0 {
		if btn.leaf {
			btn.nodeData = append(btn.nodeData[:i], btn.nodeData[i+1:]...)
			return true
		}
		leftChild := btn.children[i]
		rightChild := btn.children[i+1]
		if len(leftChild.nodeData) >= t {
			pred := leftChild.getMax()
			btn.nodeData[i] = pred
			return leftChild.delete(pred, t, cmp)
		}
		if len(rightChild.nodeData) >= t {
			succ := rightChild.getMin()
			btn.nodeData[i] = succ
			return rightChild.delete(succ, t, cmp)
		}
		btn.merge(i)
		return leftChild.delete(key, t, cmp)
	}

	if btn.leaf {
		return false
	}

	child := btn.children[i]
	if len(child.nodeData) < t {
		btn.fill(i, t)
	}
	if i > len(btn.nodeData) {
		return btn.children[i-1].delete(key, t, cmp)
	}
	return btn.children[i].delete(key, t, cmp)
}

func (btn *BTreeNode[T]) getMin() T {
	current := btn
	for !current.leaf {
		current = current.children[0]
	}
	return current.nodeData[0]
}

func (btn *BTreeNode[T]) getMax() T {
	current := btn
	for !current.leaf {
		current = current.children[len(current.children)-1]
	}
	return current.nodeData[len(current.nodeData)-1]
}

func (bt *BTree[T]) MarkDeleted(val T, setDeleted func(*T)) {
	setDeleted(&val)
	bt.Insert(val)
}

func (btn *BTreeNode[T]) fill(i int, t int) {
	if i > 0 && len(btn.children[i-1].nodeData) >= t {
		btn.borrowFromPrev(i)
	} else if i < len(btn.children)-1 && len(btn.children[i+1].nodeData) >= t {
		btn.borrowFromNext(i)
	} else {
		if i < len(btn.children)-1 {
			btn.merge(i)
		} else {
			btn.merge(i - 1)
		}
	}
}

func (btn *BTreeNode[T]) borrowFromPrev(i int) {
	child := btn.children[i]
	sibling := btn.children[i-1]

	child.nodeData = append([]T{btn.nodeData[i-1]}, child.nodeData...)
	btn.nodeData[i-1] = sibling.nodeData[len(sibling.nodeData)-1]
	sibling.nodeData = sibling.nodeData[:len(sibling.nodeData)-1]

	if !child.leaf {
		child.children = append([]*BTreeNode[T]{sibling.children[len(sibling.children)-1]}, child.children...)
		sibling.children = sibling.children[:len(sibling.children)-1]
	}
}

func (btn *BTreeNode[T]) borrowFromNext(i int) {
	child := btn.children[i]
	sibling := btn.children[i+1]

	child.nodeData = append(child.nodeData, btn.nodeData[i])
	btn.nodeData[i] = sibling.nodeData[0]
	sibling.nodeData = sibling.nodeData[1:]

	if !child.leaf {
		child.children = append(child.children, sibling.children[0])
		sibling.children = sibling.children[1:]
	}
}

func (btn *BTreeNode[T]) merge(i int) {
	child := btn.children[i]
	sibling := btn.children[i+1]
	child.nodeData = append(child.nodeData, btn.nodeData[i])
	child.nodeData = append(child.nodeData, sibling.nodeData...)
	if !child.leaf {
		child.children = append(child.children, sibling.children...)
	}
	btn.nodeData = append(btn.nodeData[:i], btn.nodeData[i+1:]...)
	btn.children = append(btn.children[:i+1], btn.children[i+2:]...)
}

func (bt *BTree[T]) EntriesInOrder() []T {
	it := bt.Iterator()
	it.SeekToFirst()
	var result []T
	for it.Valid() {
		result = append(result, it.Value())
		it.Next()
	}
	return result
}

func (bt *BTree[T]) Size() int {
	return bt.size
}

func (bt *BTree[T]) Reset() {
	nodeEntry := &BTreeNode[T]{
		nodeData: make([]T, 0),
		leaf:     true,
		children: make([]*BTreeNode[T], 0),
	}
	bt.root = nodeEntry
	bt.size = 0
}

func (bt *BTree[T]) Iterator() *BTreeIterator[T] {
	return &BTreeIterator[T]{
		tree:           bt,
		currentNode:    nil,
		indexInNode:    -1,
		stackOfParents: Stack[stackEntry[T]]{data: make([]stackEntry[T], 0)},
	}
}

func (it *BTreeIterator[T]) Valid() bool {
	return it.currentNode != nil && it.indexInNode != -1
}

func (it *BTreeIterator[T]) SeekToFirst() {
	it.stackOfParents.Clear()
	node := it.tree.root
	if node == nil {
		it.currentNode = nil
		it.indexInNode = -1
		return
	}
	for !node.leaf {
		it.stackOfParents.Push(stackEntry[T]{node, 0})
		node = node.children[0]
	}
	it.currentNode = node
	it.indexInNode = 0
}

func (it *BTreeIterator[T]) SeekToLast() {
	node := it.tree.root
	if node == nil {
		it.currentNode = nil
		it.indexInNode = -1
		return
	}
	it.stackOfParents.Clear()
	for !node.leaf {
		it.stackOfParents.Push(stackEntry[T]{node, len(node.children) - 1})
		node = node.children[len(node.children)-1]
	}
	it.currentNode = node
	it.indexInNode = len(node.nodeData) - 1
}

func (it *BTreeIterator[T]) Seek(target T) {
	node := it.tree.root
	if node == nil {
		it.currentNode = nil
		it.indexInNode = -1
		return
	}
	it.stackOfParents.Clear()
	for !node.leaf {
		i := 0
		for i < len(node.nodeData) && it.tree.cmp(node.nodeData[i], target) < 0 {
			i++
		}
		it.stackOfParents.Push(stackEntry[T]{node, i})
		node = node.children[i]
	}
	i := 0
	for i < len(node.nodeData) && it.tree.cmp(node.nodeData[i], target) < 0 {
		i++
	}
	if i < len(node.nodeData) {
		it.currentNode = node
		it.indexInNode = i
	} else {
		it.currentNode = nil
		it.indexInNode = -1
	}
}

func (it *BTreeIterator[T]) Next() {
	if it.currentNode == nil {
		return
	}
	if !it.currentNode.leaf {
		it.stackOfParents.Push(stackEntry[T]{it.currentNode, it.indexInNode + 1})
		node := it.currentNode.children[it.indexInNode+1]
		for !node.leaf {
			it.stackOfParents.Push(stackEntry[T]{node, 0})
			node = node.children[0]
		}
		it.currentNode = node
		it.indexInNode = 0
		return
	}
	if it.indexInNode+1 < len(it.currentNode.nodeData) {
		it.indexInNode++
		return
	}
	for !it.stackOfParents.Empty() {
		top := it.stackOfParents.Pop()
		parent := top.node
		idx := top.index
		if idx < len(parent.nodeData) {
			it.currentNode = parent
			it.indexInNode = idx
			return
		}
	}
	it.currentNode = nil
	it.indexInNode = -1
}

func (it *BTreeIterator[T]) Prev() {
	if it.currentNode == nil {
		return
	}
	if it.indexInNode-1 >= 0 {
		it.indexInNode--
		return
	}
	for !it.stackOfParents.Empty() {
		top := it.stackOfParents.Pop()
		parent := top.node
		idx := top.index
		if idx > 0 {
			node := parent.children[idx-1]
			it.stackOfParents.Push(stackEntry[T]{parent, idx - 1})
			for !node.leaf {
				it.stackOfParents.Push(stackEntry[T]{node, len(node.children) - 1})
				node = node.children[len(node.children)-1]
			}
			it.currentNode = node
			it.indexInNode = len(node.nodeData) - 1
			return
		}
	}
	it.currentNode = nil
	it.indexInNode = -1
}

func (it *BTreeIterator[T]) Key() T {
	if it.currentNode == nil {
		var zero T
		return zero
	}
	return it.currentNode.nodeData[it.indexInNode]
}

func (it *BTreeIterator[T]) Value() T {
	if it.currentNode == nil {
		var zero T
		return zero
	}
	return it.currentNode.nodeData[it.indexInNode]
}

func (bt *BTree[T]) Visualize(formatter func(T) string) string {
	if bt.root == nil || len(bt.root.nodeData) == 0 {
		return "BTree empty.\n"
	}
	var result strings.Builder
	result.WriteString("\n========== B-TREE ==========\n")
	type levelNode struct {
		node  *BTreeNode[T]
		level int
	}
	queue := []levelNode{{bt.root, 0}}
	currentLevel := 0
	for len(queue) > 0 {
		ln := queue[0]
		queue = queue[1:]
		if ln.level != currentLevel {
			result.WriteString("\n")
			currentLevel = ln.level
		}
		result.WriteString("[ ")
		for i, val := range ln.node.nodeData {
			result.WriteString(formatter(val))
			if i != len(ln.node.nodeData)-1 {
				result.WriteString(" | ")
			}
		}
		result.WriteString(" ] ")
		if !ln.node.leaf {
			for _, child := range ln.node.children {
				queue = append(queue, levelNode{child, ln.level + 1})
			}
		}
	}
	result.WriteString("\n============================\n")
	return result.String()
}
