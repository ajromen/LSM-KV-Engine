package data_structures

type Comparator[T any] func(a, b T) int

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

func (btn *BTreeNode[T]) search(key T, cmp Comparator[T]) (T, bool) {
	i := 0
	for i < len(btn.nodeData) && cmp(key, btn.nodeData[i]) > 0 {
		i++
	}
	if i < len(btn.nodeData) && cmp(key, btn.nodeData[i]) == 0 {
		return btn.nodeData[i], true
	}
	if btn.leaf {
		var zero T
		return zero, false
	}
	return btn.children[i].search(key, cmp)
}

func (bt *BTree[T]) SearchTree(key T) (T, bool) {
	entry, succ := bt.root.search(key, bt.cmp)
	return entry, succ
}

func (btn *BTreeNode[T]) insertNonFull(val T, t int, cmp Comparator[T]) {
	i := len(btn.nodeData) - 1
	if btn.leaf {
		for j := 0; j < len(btn.nodeData); j++ {
			if cmp(btn.nodeData[j], val) == 0 {
				btn.nodeData[j] = val
				return
			}
		}
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

func (bt *BTree[T]) MarkDeleted(val T, setDeleted func(*T)) {
	setDeleted(&val)
	bt.Insert(val)
}

func (bt *BTree[T]) EntriesInOrder() []T {
	result := make([]T, 0, bt.size)
	bt.root.inOrder(&result)
	return result
}

func (bt *BTree[T]) Size() int {
	return bt.size
}
