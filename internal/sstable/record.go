package sstable

import "github.com/ajromen/LSM-KV-Engine/internal/enums"

type Record struct {
	SeqId     uint64
	OpType    enums.OpType
	ExpiresAt int64
	Key       []byte
	Value     []byte
}
