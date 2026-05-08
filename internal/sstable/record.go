package sstable

import "github.com/ajromen/LSM-KV-Engine/internal/enums"

const (
	ChunkTypeFull   byte = 0 // entire record fits in one block
	ChunkTypeFirst  byte = 1 // first chunk of a split record
	ChunkTypeMiddle byte = 2 // middle chunk
	ChunkTypeLast   byte = 3 // last chunk
)

type Record struct {
	SeqId     uint64
	OpType    enums.OpType
	ExpiresAt int64
	Key       []byte
	Value     []byte
	ChunkType byte
}
