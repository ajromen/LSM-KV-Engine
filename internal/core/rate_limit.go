package core

import (
	"fmt"

	"github.com/ajromen/LSM-KV-Engine/internal/enums"
	"github.com/ajromen/LSM-KV-Engine/internal/token_bucket"
)

func (engine *Engine) persistTokenBucket() {
	if engine.tokenBucket == nil {
		return
	}
	seqId := engine.seqGen.Next()
	serialized := engine.tokenBucket.Serialize()
	engine.wal.Put([]byte(token_bucket.InternalKey), serialized, seqId, enums.OpTypePut)
	engine.lsm.Put([]byte(token_bucket.InternalKey), serialized, seqId, enums.OpTypePut)
}

func (engine *Engine) checkRateLimit() error {
	if engine.tokenBucket == nil || !engine.tokenBucket.IsEnabled() {
		return nil
	}
	if !engine.tokenBucket.TryConsume() {
		return fmt.Errorf("rate limit exceeded: too many requests")
	}
	engine.persistTokenBucket()
	return nil
}
