package ttl

import "github.com/ajromen/LSM-KV-Engine/internal/sstable"

type ExpiryHeap struct {
	entries []sstable.TTLEntry
}

func NewExpiryHeap() *ExpiryHeap {
	return &ExpiryHeap{}
}
