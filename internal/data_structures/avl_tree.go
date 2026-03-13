package data_structures

import "strings"

type AVLTreeNode[T any] struct {
	Key    T
	left   *AVLTreeNode[T]
	right  *AVLTreeNode[T]
	height int
}

type AVLTree[T any] struct {
	root                 *AVLTreeNode[T]
	cmp                  Comparator[T]
	cmpIgnoringTimestamp Comparator[T]
}

type avlStackEntry[T any] struct {
	node         *AVLTreeNode[T]
	cameFromLeft bool
}

type AVLTreeIterator[T any] struct {
	tree           *AVLTree[T]
	current        *AVLTreeNode[T]
	stackOfParents Stack[avlStackEntry[T]]
}

func NewAVLTreeNode[T any](key T) *AVLTreeNode[T] {
	return &AVLTreeNode[T]{
		Key:    key,
		left:   nil,
		right:  nil,
		height: 1,
	}
}

func NewAVLTree[T any](cmp Comparator[T], cmpIgnoringTimestamp Comparator[T]) *AVLTree[T] {
	return &AVLTree[T]{
		root:                 nil,
		cmp:                  cmp,
		cmpIgnoringTimestamp: cmpIgnoringTimestamp,
	}
}

func height[T any](node *AVLTreeNode[T]) int {
	if node == nil {
		return 0
	}
	return node.height
}

func (t *AVLTree[T]) leftRotate(x *AVLTreeNode[T]) *AVLTreeNode[T] {
	if x == nil || x.right == nil {
		return x
	}
	y := x.right
	x.right = y.left
	y.left = x
	x.height = max(height(x.left), height(x.right)) + 1
	y.height = max(height(y.left), height(y.right)) + 1
	return y
}

func (t *AVLTree[T]) rightRotate(x *AVLTreeNode[T]) *AVLTreeNode[T] {
	if x == nil || x.left == nil {
		return x
	}
	y := x.left
	x.left = y.right
	y.right = x
	x.height = max(height(x.left), height(x.right)) + 1
	y.height = max(height(y.left), height(y.right)) + 1
	return y
}

func (t *AVLTree[T]) leftRightRotate(x *AVLTreeNode[T]) *AVLTreeNode[T] {
	if x == nil || x.left == nil {
		return x
	}
	x.left = t.leftRotate(x.left)
	return t.rightRotate(x)
}

func (t *AVLTree[T]) rightLeftRotate(x *AVLTreeNode[T]) *AVLTreeNode[T] {
	if x == nil || x.right == nil {
		return x
	}
	x.right = t.rightRotate(x.right)
	return t.leftRotate(x)
}

func (t *AVLTree[T]) insertNode(node *AVLTreeNode[T], key T) *AVLTreeNode[T] {
	if node == nil {
		return NewAVLTreeNode(key)
	}
	if t.cmp(key, node.Key) <= 0 {
		node.left = t.insertNode(node.left, key)
	} else {
		node.right = t.insertNode(node.right, key)
	}
	node.height = max(height(node.left), height(node.right)) + 1
	balance := height(node.left) - height(node.right)
	if balance > 1 && t.cmp(key, node.left.Key) <= 0 {
		return t.rightRotate(node)
	}
	if balance > 1 && t.cmp(key, node.left.Key) > 0 {
		return t.leftRightRotate(node)
	}
	if balance < -1 && t.cmp(key, node.right.Key) > 0 {
		return t.leftRotate(node)
	}
	if balance < -1 && t.cmp(key, node.right.Key) <= 0 {
		return t.rightLeftRotate(node)
	}
	return node
}

func (t *AVLTree[T]) Insert(key T) {
	t.root = t.insertNode(t.root, key)
}

func (t *AVLTree[T]) LowerBound(key T) *AVLTreeNode[T] {
	node := t.root
	var candidate *AVLTreeNode[T]
	for node != nil {
		if t.cmp(node.Key, key) >= 0 {
			candidate = node
			node = node.left
		} else {
			node = node.right
		}
	}
	return candidate
}

func (t *AVLTree[T]) UpperBound(key T) *AVLTreeNode[T] {
	node := t.root
	var candidate *AVLTreeNode[T]
	for node != nil {
		if t.cmp(node.Key, key) > 0 {
			candidate = node
			node = node.left
		} else {
			node = node.right
		}
	}
	return candidate
}

func (t *AVLTree[T]) deleteNode(node *AVLTreeNode[T], key T) *AVLTreeNode[T] {
	if node == nil {
		return nil
	}
	if t.cmp(key, node.Key) < 0 {
		node.left = t.deleteNode(node.left, key)
	} else if t.cmp(key, node.Key) > 0 {
		node.right = t.deleteNode(node.right, key)
	} else {
		if node.left == nil || node.right == nil {
			var temp *AVLTreeNode[T]
			if node.left != nil {
				temp = node.left
			} else {
				temp = node.right
			}
			if temp == nil {
				node = nil
			} else {
				node = temp
			}
		} else {
			successor := node.right
			for successor.left != nil {
				successor = successor.left
			}
			node.Key = successor.Key
			node.right = t.deleteNode(node.right, successor.Key)
		}
	}
	if node == nil {
		return nil
	}
	node.height = max(height(node.left), height(node.right)) + 1
	balance := height(node.left) - height(node.right)
	if balance > 1 && height(node.left.left)-height(node.left.right) >= 0 {
		return t.rightRotate(node)
	}
	if balance > 1 && height(node.left.left)-height(node.left.right) < 0 {
		return t.leftRightRotate(node)
	}
	if balance < -1 && height(node.right.left)-height(node.right.right) <= 0 {
		return t.leftRotate(node)
	}
	if balance < -1 && height(node.right.left)-height(node.right.right) > 0 {
		return t.rightLeftRotate(node)
	}
	return node
}

func (t *AVLTree[T]) Delete(key T) {
	t.root = t.deleteNode(t.root, key)
}

func (t *AVLTree[T]) EntriesInOrder() []T {
	result := make([]T, 0)
	t.inOrderHelper(t.root, &result)
	return result
}

func (t *AVLTree[T]) inOrderHelper(node *AVLTreeNode[T], result *[]T) {
	if node == nil {
		return
	}
	t.inOrderHelper(node.left, result)
	*result = append(*result, node.Key)
	t.inOrderHelper(node.right, result)
}

func (t *AVLTree[T]) Reset() {
	t.root = nil
}

func (t *AVLTree[T]) Size() int {
	return t.sizeHelper(t.root)
}

func (t *AVLTree[T]) sizeHelper(node *AVLTreeNode[T]) int {
	if node == nil {
		return 0
	}
	return 1 + t.sizeHelper(node.left) + t.sizeHelper(node.right)
}

func (t *AVLTree[T]) Iterator() *AVLTreeIterator[T] {
	return &AVLTreeIterator[T]{
		tree:    t,
		current: nil,
	}
}

func (it *AVLTreeIterator[T]) Valid() bool {
	return it.current != nil
}

func (it *AVLTreeIterator[T]) SeekToFirst() {
	it.stackOfParents.Clear()
	node := it.tree.root
	for node != nil && node.left != nil {
		it.stackOfParents.Push(avlStackEntry[T]{node, true})
		node = node.left
	}
	it.current = node
}

func (it *AVLTreeIterator[T]) SeekToLast() {
	it.stackOfParents.Clear()
	node := it.tree.root
	for node != nil && node.right != nil {
		it.stackOfParents.Push(avlStackEntry[T]{node, false})
		node = node.right
	}
	it.current = node
}

func (it *AVLTreeIterator[T]) Next() {
	if it.current == nil {
		return
	}
	if it.current.right != nil {
		it.stackOfParents.Push(avlStackEntry[T]{it.current, false})
		node := it.current.right
		for node.left != nil {
			it.stackOfParents.Push(avlStackEntry[T]{node, true})
			node = node.left
		}
		it.current = node
		return
	}
	for it.stackOfParents.Len() > 0 {
		entry := it.stackOfParents.Pop()
		if entry.cameFromLeft {
			it.current = entry.node
			return
		}
	}
	it.current = nil
}

func (it *AVLTreeIterator[T]) Prev() {
	if it.current == nil {
		return
	}
	if it.current.left != nil {
		it.stackOfParents.Push(avlStackEntry[T]{it.current, true})
		node := it.current.left
		for node.right != nil {
			it.stackOfParents.Push(avlStackEntry[T]{node, false})
			node = node.right
		}
		it.current = node
		return
	}
	for it.stackOfParents.Len() > 0 {
		entry := it.stackOfParents.Pop()
		if !entry.cameFromLeft {
			it.current = entry.node
			return
		}
	}
	it.current = nil
}

func (it *AVLTreeIterator[T]) Seek(key T) {
	it.stackOfParents.Clear()
	it.current = nil
	node := it.tree.root
	for node != nil {
		cmp := it.tree.cmpIgnoringTimestamp(node.Key, key)
		if cmp >= 0 {
			if node.left != nil {
				it.stackOfParents.Push(avlStackEntry[T]{node, true})
			}
			it.current = node
			node = node.left
		} else {
			if node.right != nil {
				it.stackOfParents.Push(avlStackEntry[T]{node, false})
			}
			node = node.right
		}
	}
}

func (it *AVLTreeIterator[T]) Key() T {
	if it.current == nil {
		var zero T
		return zero
	}
	return it.current.Key
}

func (it *AVLTreeIterator[T]) Value() T {
	if it.current == nil {
		var zero T
		return zero
	}
	return it.current.Key
}

func (t *AVLTree[T]) Visualize(formatter func(T) string) string {
	if t.root == nil {
		return "AVLTree empty.\n"
	}
	var result strings.Builder
	result.WriteString("\n========== AVL TREE ==========\n")

	type levelNode struct {
		node  *AVLTreeNode[T]
		level int
	}

	queue := []levelNode{{t.root, 0}}
	currentLevel := 0

	for len(queue) > 0 {
		ln := queue[0]
		queue = queue[1:]

		if ln.level != currentLevel {
			result.WriteString("\n")
			currentLevel = ln.level
		}

		result.WriteString("[ ")
		result.WriteString(formatter(ln.node.Key))
		result.WriteString(" ] ")

		if ln.node.left != nil {
			queue = append(queue, levelNode{ln.node.left, ln.level + 1})
		}
		if ln.node.right != nil {
			queue = append(queue, levelNode{ln.node.right, ln.level + 1})
		}
	}

	result.WriteString("\n=============================\n")
	return result.String()
}
