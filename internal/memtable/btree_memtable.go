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
	size int
}

func NewBTree(t int) *BTree {
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
func (btn *BTreeNode) insertNonFull(entry MemtableEntry, t int) {
	i := len(btn.nodeData) - 1
	if btn.leaf {
		for j := 0; j < len(btn.nodeData); j++ {
			if btn.nodeData[j].Key == entry.Key {
				btn.nodeData[j] = entry
				return
			}
		}
		btn.nodeData = append(btn.nodeData, MemtableEntry{})
		for i >= 0 && entry.Key < btn.nodeData[i].Key {
			btn.nodeData[i+1] = btn.nodeData[i]
			i--
		}
		btn.nodeData[i+1] = entry
		return
	}
	for i >= 0 && entry.Key < btn.nodeData[i].Key {
		i--
	}
	i++
	if len(btn.children[i].nodeData) == 2*t-1 {
		btn.splitChild(i, t)
		if entry.Key > btn.nodeData[i].Key {
			i++
		}
	}
	btn.children[i].insertNonFull(entry, t)
}

func (btn *BTreeNode) splitChild(i int, t int) {
	fullNode := btn.children[i]
	newNode := &BTreeNode{
		nodeData: make([]MemtableEntry, t-1),
		leaf:     fullNode.leaf,
	}
	for j := 0; j < t-1; j++ {
		newNode.nodeData[j] = fullNode.nodeData[j+t]
	}
	if !fullNode.leaf {
		newNode.children = make([]*BTreeNode, t)
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
	btn.nodeData = append(btn.nodeData, MemtableEntry{})
	for j := len(btn.nodeData) - 1; j > i; j-- {
		btn.nodeData[j] = btn.nodeData[j-1]
	}
	btn.nodeData[i] = median
}

func (btn *BTreeNode) inOrder(result *[]MemtableEntry) {
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

func (bt *BTree) Insert(entry MemtableEntry) {
	root := bt.root
	if len(root.nodeData) == 2*bt.t-1 {
		s := &BTreeNode{leaf: false, children: []*BTreeNode{root}}
		bt.root = s
		s.splitChild(0, bt.t)
		s.insertNonFull(entry, bt.t)
	} else {
		root.insertNonFull(entry, bt.t)
	}
	bt.size += 1
}

func (bt *BTree) markDeleted(key string) {
	entry := MemtableEntry{
		Key:       key,
		Value:     nil,
		Tombstone: true,
	}
	bt.Insert(entry)
}

func (bt *BTree) entriesInOrder() []MemtableEntry {
	result := make([]MemtableEntry, 0, bt.size)
	bt.root.inOrder(&result)
	return result
}

type BTreeMemtable struct {
	memtableData *BTree
	maxSize      int
	flushHandler func([]MemtableEntry)
}

func NewBTreeMem(maxSize int, flushHandler func([]MemtableEntry)) *BTreeMemtable {
	return &BTreeMemtable{
		memtableData: NewBTree(8),
		maxSize:      maxSize,
		flushHandler: flushHandler,
	}
}

func (memtable *BTreeMemtable) Put(key string, value []byte) {
	shouldFlush := memtable.Flush()
	if shouldFlush {
		entries := memtable.FlushEntries()
		if memtable.flushHandler != nil {
			memtable.flushHandler(entries)
		}
	}
	entry := MemtableEntry{
		Key:       key,
		Value:     value,
		Tombstone: false,
	}
	memtable.memtableData.Insert(entry)
}

func (memtable *BTreeMemtable) Get(key string) (MemtableEntry, bool) {
	return memtable.memtableData.SearchTree(key)
}

func (memtable *BTreeMemtable) Delete(key string) {
	memtable.memtableData.markDeleted(key)
}

func (memtable *BTreeMemtable) Flush() bool {
	return memtable.memtableData.size >= memtable.maxSize
}

func (memtable *BTreeMemtable) Reset() {
	memtable.memtableData = NewBTree(8)
}

func (memtable *BTreeMemtable) FlushEntries() []MemtableEntry {
	entries := memtable.memtableData.entriesInOrder()
	memtable.Reset()
	return entries
}

func (memtable *BTreeMemtable) ReadEntriesNoFlushing() []MemtableEntry {
	entries := memtable.memtableData.entriesInOrder()
	return entries
}
