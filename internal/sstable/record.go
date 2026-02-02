package sstable

type Record struct {
	Key       string
	Value     []byte
	Tombstone bool
	Type      string
}
