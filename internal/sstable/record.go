package sstable

type Record struct {
	SeqId     uint64
	Tombstone bool
	Key       []byte
	Value     []byte
}
