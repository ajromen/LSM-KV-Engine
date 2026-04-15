package sstable

type Record struct {
	SeqId     uint64
	Tombstone bool
	ExpiresAt int64
	Key       []byte
	Value     []byte
}
