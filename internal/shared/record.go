package shared

import "github.com/ajromen/LSM-KV-Engine/internal/enums"

// NOT IN USE
type Record struct {
	OpType enums.OpType // type of operation being saved -> older version had tombstone now it is OpType=opTypeDelete=1

	Key   []byte // Raw key (in case of opTypeRangeDel = start key -> lower bound of given range)
	Value []byte // Raw value (in case of opTypeDelete = nil | in case of opTypeMerge = deltaOperation (+1 for example) | in case of opTypeRangeDel = endKey)

	SeqId     uint64 // sequence number -> every nonatomic operation has its own sequence number
	ExpiresAt int64  // timestamp when key expires ( timestamp(now) + given ttl)
}
