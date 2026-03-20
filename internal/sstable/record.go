package sstable

import "github.com/ajromen/LSM-KV-Engine/internal/utils"

type Record struct {
	Timestamp utils.Uint128
	Tombstone bool
	Key       []byte
	Value     []byte
}
