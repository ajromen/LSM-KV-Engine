package memtable

type Node struct {
	key   string
	value []byte
	next  []*Node
}

type SkipList struct {
	header *Node
	level  int
}

func NewNode(key string, value []byte, level int) *Node {
	return &Node{
		key:   key,
		value: value,
		next:  make([]*Node, level),
	}
}

func NewSkipList(MaxLevel int) *SkipList {
	return &SkipList{
		header: NewNode("", []byte{}, MaxLevel),
		level:  MaxLevel,
	}
}
