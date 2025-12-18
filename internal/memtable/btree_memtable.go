package memtable

type BTreeNode struct {
	nodeData []MemtableEntry
	leaf     bool
	children []*BTreeNode
}

/*
t - minimum degree
Svaki cvor ima najmanje t-1 kljuceva, a najvise 2t-1 kljuceva
Svaki cvor ima najvise 2t dece
*/

type BTree struct {
	root *BTreeNode
	t    int
}

func newBTree(t int) *BTree {
	nodeEntry := &BTreeNode{
		nodeData: make([]MemtableEntry, 0),
		leaf:     true,
		children: make([]*BTreeNode, 0),
	}
	return &BTree{
		root: nodeEntry,
		t:    t,
	}
}

func (btn *BTreeNode) Search(key string) MemtableEntry {
	i := 0
	for i < len(btn.nodeData) && btn.nodeData[i].Key < key {
		i++
	}
	if i < len(btn.nodeData) && btn.nodeData[i].Key == key {
		return btn.nodeData[i]
	}
	if btn.leaf {
		return MemtableEntry{}
	}
	return btn.children[i].Search(key)
}

func (bt *BTree) SearchTree(key string) (MemtableEntry, bool) {
	entry := bt.root.Search(key)
	if entry.Tombstone || entry.Value == nil || entry.Key == "" {
		return MemtableEntry{}, false
	}
	return entry, true
}
