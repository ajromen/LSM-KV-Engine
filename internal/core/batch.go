package core

import (
	"fmt"

	"github.com/ajromen/LSM-KV-Engine/internal/enums"
	"github.com/ajromen/LSM-KV-Engine/internal/token_bucket"
	"github.com/ajromen/LSM-KV-Engine/internal/wal"
)

type KeyValue struct {
	Key   []byte
	Value []byte
}

func (engine *Engine) BatchWrite(pairs []KeyValue) error {
	if len(pairs) == 0 {
		return nil
	}
	if err := engine.checkRateLimit(); err != nil {
		return err
	}
	for i, kv := range pairs {
		if len(kv.Key) == 0 {
			return fmt.Errorf("batch write: empty key at index %d", i)
		}
		if string(kv.Key) == token_bucket.InternalKey {
			return fmt.Errorf("batch write: reserved key at index %d", i)
		}
	}
	seqIds := make([]uint64, len(pairs))
	for i := range pairs {
		seqIds[i] = engine.seqGen.Next()
	}
	ops := make([]wal.TxnOp, len(pairs))
	for i, kv := range pairs {
		ops[i] = wal.TxnOp{
			SeqId:     seqIds[i],
			ExpiresAt: 0,
			OpType:    enums.OpTypePut,
			Key:       kv.Key,
			Value:     kv.Value,
		}
	}
	if err := engine.wal.BatchWrite(ops); err != nil {
		return fmt.Errorf("batch write: WAL failed, nothing applied: %w", err)
	}
	applied := 0
	var applyErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				applyErr = fmt.Errorf("batch write panicked: %v", r)
			}
		}()
		for i, kv := range pairs {
			engine.lsm.Put(kv.Key, kv.Value, seqIds[i], enums.OpTypePut)
			applied++
		}
	}()

	if applyErr != nil {
		for i := 0; i < applied; i++ {
			engine.lsm.Remove(pairs[i].Key, seqIds[i])
		}
		return fmt.Errorf("batch write: partial apply rolled back (%d/%d): %w", applied, len(pairs), applyErr)
	}

	for _, kv := range pairs {
		engine.notifier.NotifyPut(kv.Key, kv.Value)
	}
	return nil
}

func (engine *Engine) BatchDelete(keys [][]byte) error {
	if len(keys) == 0 {
		return nil
	}
	if err := engine.checkRateLimit(); err != nil {
		return err
	}
	for i, key := range keys {
		if len(key) == 0 {
			return fmt.Errorf("batch delete: empty key at index %d", i)
		}
		if string(key) == token_bucket.InternalKey {
			return fmt.Errorf("batch delete: reserved key at index %d", i)
		}
	}

	seqIds := make([]uint64, len(keys))
	for i := range keys {
		seqIds[i] = engine.seqGen.Next()
	}

	ops := make([]wal.TxnOp, len(keys))
	for i, key := range keys {
		ops[i] = wal.TxnOp{
			SeqId:     seqIds[i],
			ExpiresAt: 0,
			OpType:    enums.OpTypeDel,
			Key:       key,
			Value:     nil,
		}
	}
	if err := engine.wal.BatchWrite(ops); err != nil {
		return fmt.Errorf("batch delete: WAL failed, nothing applied: %w", err)
	}

	applied := 0
	var applyErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				applyErr = fmt.Errorf("batch delete panicked: %v", r)
			}
		}()
		for i, key := range keys {
			engine.lsm.Put(key, nil, seqIds[i], enums.OpTypeDel)
			applied++
		}
	}()

	if applyErr != nil {
		for i := 0; i < applied; i++ {
			engine.lsm.Remove(keys[i], seqIds[i])
		}
		return fmt.Errorf("batch delete: partial apply rolled back (%d/%d): %w", applied, len(keys), applyErr)
	}

	for _, key := range keys {
		engine.notifier.NotifyDelete(key)
	}
	return nil
}
