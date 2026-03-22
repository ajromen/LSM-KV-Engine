package data_structures

import (
	"strings"
)

const (
	Red   byte = 0
	Black byte = 1
)

type RBTreeNode[T any] struct {
	Key    T
	left   *RBTreeNode[T]
	right  *RBTreeNode[T]
	parent *RBTreeNode[T]
	color  byte
}

type RBTree[T any] struct {
	root                 *RBTreeNode[T]
	sentinel             *RBTreeNode[T]
	cmp                  Comparator[T]
	cmpIgnoringTimestamp Comparator[T]
	size                 int
}

type RBTreeIterator[T any] struct {
	tree    *RBTree[T]
	current *RBTreeNode[T]
}

func NewRBTree[T any](cmp Comparator[T], cmpIgnoringTimestamp Comparator[T]) *RBTree[T] {
	nilNode := &RBTreeNode[T]{color: Black}
	nilNode.left = nilNode
	nilNode.right = nilNode
	nilNode.parent = nilNode
	return &RBTree[T]{
		root:                 nilNode,
		sentinel:             nilNode,
		cmp:                  cmp,
		cmpIgnoringTimestamp: cmpIgnoringTimestamp,
	}
}

func (t *RBTree[T]) leftRotate(x *RBTreeNode[T]) {
	y := x.right
	x.right = y.left
	if y.left != t.sentinel {
		y.left.parent = x
	}
	y.parent = x.parent
	if x.parent == t.sentinel {
		t.root = y
	} else if x == x.parent.left {
		x.parent.left = y
	} else {
		x.parent.right = y
	}
	y.left = x
	x.parent = y
}

func (t *RBTree[T]) rightRotate(x *RBTreeNode[T]) {
	y := x.left
	x.left = y.right
	if y.right != t.sentinel {
		y.right.parent = x
	}
	y.parent = x.parent
	if x.parent == t.sentinel {
		t.root = y
	} else if x == x.parent.right {
		x.parent.right = y
	} else {
		x.parent.left = y
	}
	y.right = x
	x.parent = y
}

func (t *RBTree[T]) LowerBound(key T) *RBTreeNode[T] {
	node := t.root
	var candidate *RBTreeNode[T]
	for node != t.sentinel {
		cmp := t.cmpIgnoringTimestamp(node.Key, key)
		if cmp >= 0 {
			candidate = node
			node = node.left
		} else {
			node = node.right
		}
	}
	for candidate != nil && candidate.left != t.sentinel &&
		t.cmpIgnoringTimestamp(candidate.left.Key, key) == 0 {
		candidate = candidate.left
	}
	return candidate
}

func (t *RBTree[T]) UpperBound(key T) *RBTreeNode[T] {
	current := t.root
	var result *RBTreeNode[T]
	for current != t.sentinel {
		if t.cmp(current.Key, key) > 0 {
			result = current
			current = current.left
		} else {
			current = current.right
		}
	}
	return result
}

func (t *RBTree[T]) Insert(key T) {
	z := &RBTreeNode[T]{
		Key:   key,
		color: Red,
		left:  t.sentinel,
		right: t.sentinel,
	}
	y := t.sentinel
	x := t.root
	for x != t.sentinel {
		y = x
		if t.cmp(z.Key, x.Key) < 0 {
			x = x.left
		} else {
			x = x.right
		}
	}
	z.parent = y
	if y == t.sentinel {
		t.root = z
	} else if t.cmp(z.Key, y.Key) < 0 {
		y.left = z
	} else {
		y.right = z
	}
	t.size++
	t.insertFixup(z)
}

func (t *RBTree[T]) insertFixup(z *RBTreeNode[T]) {
	for z.parent.color == Red {
		if z.parent == z.parent.parent.left {
			y := z.parent.parent.right

			if y.color == Red {
				z.parent.color = Black
				y.color = Black
				z.parent.parent.color = Red
				z = z.parent.parent
			} else {
				if z == z.parent.right {
					z = z.parent
					t.leftRotate(z)
				}
				z.parent.color = Black
				z.parent.parent.color = Red
				t.rightRotate(z.parent.parent)
			}
		} else {
			y := z.parent.parent.left

			if y.color == Red {
				z.parent.color = Black
				y.color = Black
				z.parent.parent.color = Red
				z = z.parent.parent
			} else {
				if z == z.parent.left {
					z = z.parent
					t.rightRotate(z)
				}
				z.parent.color = Black
				z.parent.parent.color = Red
				t.leftRotate(z.parent.parent)
			}
		}
	}
	t.root.color = Black
}

func (t *RBTree[T]) Reset() {
	nilNode := &RBTreeNode[T]{color: Black}
	nilNode.left = nilNode
	nilNode.right = nilNode
	nilNode.parent = nilNode

	t.root = nilNode
	t.sentinel = nilNode
	t.size = 0
}

func (t *RBTree[T]) transplant(u, v *RBTreeNode[T]) {
	if u.parent == t.sentinel {
		t.root = v
	} else if u == u.parent.left {
		u.parent.left = v
	} else {
		u.parent.right = v
	}
	v.parent = u.parent
}

func (t *RBTree[T]) minimum(x *RBTreeNode[T]) *RBTreeNode[T] {
	for x.left != t.sentinel {
		x = x.left
	}
	return x
}

func (t *RBTree[T]) Delete(z *RBTreeNode[T]) {
	if z == nil || z == t.sentinel {
		return
	}
	y := z
	yOriginalColor := y.color
	var x *RBTreeNode[T]
	if z.left == t.sentinel {
		x = z.right
		t.transplant(z, z.right)
	} else if z.right == t.sentinel {
		x = z.left
		t.transplant(z, z.left)
	} else {
		y = t.minimum(z.right)
		yOriginalColor = y.color
		x = y.right

		if y.parent == z {
			x.parent = y
		} else {
			t.transplant(y, y.right)
			y.right = z.right
			y.right.parent = y
		}
		t.transplant(z, y)
		y.left = z.left
		y.left.parent = y
		y.color = z.color
	}
	if yOriginalColor == Black {
		t.deleteFixup(x)
	}
}

func (t *RBTree[T]) deleteFixup(x *RBTreeNode[T]) {
	for x != t.root && x.color == Black {
		if x == x.parent.left {
			w := x.parent.right
			if w.color == Red {
				w.color = Black
				x.parent.color = Red
				t.leftRotate(x.parent)
				w = x.parent.right
			}
			if w.left.color == Black && w.right.color == Black {
				w.color = Red
				x = x.parent
			} else {
				if w.right.color == Black {
					w.left.color = Black
					w.color = Red
					t.rightRotate(w)
					w = x.parent.right
				}
				w.color = x.parent.color
				x.parent.color = Black
				w.right.color = Black
				t.leftRotate(x.parent)
				x = t.root
			}
		} else {
			w := x.parent.left
			if w.color == Red {
				w.color = Black
				x.parent.color = Red
				t.rightRotate(x.parent)
				w = x.parent.left
			}
			if w.right.color == Black && w.left.color == Black {
				w.color = Red
				x = x.parent
			} else {
				if w.left.color == Black {
					w.right.color = Black
					w.color = Red
					t.leftRotate(w)
					w = x.parent.left
				}
				w.color = x.parent.color
				x.parent.color = Black
				w.left.color = Black
				t.rightRotate(x.parent)
				x = t.root
			}
		}
	}
	x.color = Black
}

func (t *RBTree[T]) EntriesInOrder() []T {
	it := t.Iterator()
	it.SeekToFirst()
	result := make([]T, 0, t.size)
	for it.Valid() {
		result = append(result, it.Value())
		it.Next()
	}
	return result
}

func (t *RBTree[T]) inOrderHelper(node *RBTreeNode[T], result *[]T) {
	if node == t.sentinel {
		return
	}
	t.inOrderHelper(node.left, result)
	*result = append(*result, node.Key)
	t.inOrderHelper(node.right, result)
}

func (t *RBTree[T]) Iterator() *RBTreeIterator[T] {
	return &RBTreeIterator[T]{
		tree:    t,
		current: nil,
	}
}

func (it *RBTreeIterator[T]) Valid() bool {
	return it.current != it.tree.sentinel && it.current != nil
}

func (it *RBTreeIterator[T]) SeekToFirst() {
	node := it.tree.root
	if node == nil {
		return
	}
	for node.left != it.tree.sentinel {
		node = node.left
	}
	it.current = node
}

func (it *RBTreeIterator[T]) SeekToLast() {
	node := it.tree.root
	if node == nil {
		return
	}
	for node.right != it.tree.sentinel {
		node = node.right
	}
	it.current = node
}

func (it *RBTreeIterator[T]) Seek(key T) {
	node := it.tree.root
	var candidate *RBTreeNode[T]
	for node != it.tree.sentinel {
		cmp := it.tree.cmpIgnoringTimestamp(node.Key, key)
		if cmp >= 0 {
			candidate = node
			node = node.left
		} else {
			node = node.right
		}
	}
	for candidate != nil && candidate.left != it.tree.sentinel &&
		it.tree.cmpIgnoringTimestamp(candidate.left.Key, key) == 0 {
		candidate = candidate.left
	}
	it.current = candidate
}

func (it *RBTreeIterator[T]) Next() {
	if it.current == it.tree.sentinel {
		return
	}
	if it.current.right != it.tree.sentinel {
		it.current = it.current.right
		for it.current.left != it.tree.sentinel {
			it.current = it.current.left
		}
		return
	}
	parent := it.current.parent
	for parent != it.tree.sentinel && it.current == parent.right {
		it.current = parent
		parent = parent.parent
	}
	it.current = parent
}

func (it *RBTreeIterator[T]) Prev() {
	if it.current == it.tree.sentinel {
		return
	}
	if it.current.left != it.tree.sentinel {
		it.current = it.current.left
		for it.current.right != it.tree.sentinel {
			it.current = it.current.right
		}
		return
	}
	parent := it.current.parent
	for parent != it.tree.sentinel && it.current == parent.left {
		it.current = parent
		parent = parent.parent
	}

	it.current = parent
}

func (it *RBTreeIterator[T]) Key() T {
	if it.current == nil {
		var zero T
		return zero
	}
	return it.current.Key
}

func (it *RBTreeIterator[T]) Value() T {
	if it.current == nil {
		var zero T
		return zero
	}
	return it.current.Key
}

func (t *RBTree[T]) Visualize(formatter func(T) string) string {
	if t.root == t.sentinel {
		return "RBTree empty.\n"
	}
	var result strings.Builder
	result.WriteString("\n========== RED-BLACK TREE ==========\n")
	type levelNode struct {
		node  *RBTreeNode[T]
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
		color := "⚫"
		if ln.node.color == Red {
			color = "🔴"
		}
		result.WriteString("[ ")
		result.WriteString(formatter(ln.node.Key))
		result.WriteString(" ")
		result.WriteString(color)
		result.WriteString(" ] ")
		if ln.node.left != t.sentinel {
			queue = append(queue, levelNode{ln.node.left, ln.level + 1})
		}
		if ln.node.right != t.sentinel {
			queue = append(queue, levelNode{ln.node.right, ln.level + 1})
		}
	}
	result.WriteString("\n====================================\n")
	return result.String()
}

func (t *RBTree[T]) Size() int {
	return t.size
}
