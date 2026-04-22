package core

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
	"github.com/ajromen/LSM-KV-Engine/internal/lsm"
	"github.com/ajromen/LSM-KV-Engine/internal/notifier"
	"github.com/ajromen/LSM-KV-Engine/internal/sequence"
	"github.com/ajromen/LSM-KV-Engine/internal/shared"
	"github.com/ajromen/LSM-KV-Engine/internal/token_bucket"
	"github.com/ajromen/LSM-KV-Engine/internal/ttl"
)

func NewEngine() (*Engine, error) {
	dataDir := config.GetSettings().SavePath
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}
	lsmTree, err := lsm.NewLSM(dataDir)
	if err != nil {
		return nil, err
	}
	engine := Engine{lsm: lsmTree, inMemoryTTL: config.GetSettings().TTL.InMemoryTTL, notifier: notifier.NewNotifier()}

	engine.recover()

	cfg := config.GetSettings().TokenBucket
	if cfg.MaxTokens > 0 {
		existing, found, err := lsmTree.Get([]byte(token_bucket.InternalKey))
		if err == nil && found {
			tb := token_bucket.Deserialize(existing)
			if tb != nil {
				engine.tokenBucket = tb
			}
		}

		if engine.tokenBucket == nil {
			engine.tokenBucket = token_bucket.New(cfg.MaxTokens, cfg.ResetIntervalMs)
			engine.persistTokenBucket()
		}
	}

	if engine.inMemoryTTL {
		engine.ttlJanitor = ttl.NewTTLJanitor(engine.notifier)
		heap, index, err := engine.lsm.GetAllTTLFomSST()
		if err != nil {
			return nil, err
		}
		engine.ttlJanitor.Init(heap, index)
		go engine.ttlJanitor.Run()
	}
	if config.GetSettings().Debug {
		print("Engine created\n")
	}
	return &engine, nil
}

func (engine *Engine) persistTokenBucket() {
	if engine.tokenBucket == nil {
		return
	}
	seqId := engine.seqGen.Next()
	engine.lsm.Put([]byte(token_bucket.InternalKey), engine.tokenBucket.Serialize(), seqId, enums.OpTypePut)
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

// check manifest
// check wal
func (engine *Engine) recover() {
	var maxSeq uint64
	maxSeq = engine.lsm.GetMaxSeqId()
	//wal
	engine.seqGen = sequence.NewSequenceGenerator(maxSeq)
}

func (engine *Engine) Put(key []byte, value []byte) {
	if err := engine.checkRateLimit(); err != nil {
		fmt.Println(err)
		return
	}

	if string(key) == token_bucket.InternalKey {
		return
	}

	seqId := engine.seqGen.Next()
	//wal
	engine.lsm.Put(key, value, seqId, enums.OpTypePut)
	engine.notifier.NotifyPut(key, value)
}

func (engine *Engine) PutWithTTL(key []byte, value []byte, ttl int64) {
	seqId := engine.seqGen.Next()
	//wal
	if engine.inMemoryTTL {
		engine.ttlJanitor.AddTTL(shared.TTLEntry{ExpiresAt: time.Now().UnixMilli() + ttl, Key: key})
	}
	engine.lsm.PutWithTTL(key, value, seqId, enums.OpTypePut, ttl)
	engine.notifier.NotifyPut(key, value)
}

func (engine *Engine) Get(key []byte) ([]byte, bool, error) {
	if err := engine.checkRateLimit(); err != nil {
		return nil, false, err
	}

	if string(key) == token_bucket.InternalKey {
		return nil, false, fmt.Errorf("key not found")
	}

	value, found, err := engine.lsm.Get(key)
	return value, found, err
}

func (engine *Engine) GetTTL(key []byte) (int64, bool, error) {
	if !config.GetSettings().TTL.InMemoryTTL {
		value, found, err := engine.lsm.GetTTL(key)
		return value, found, err
	}
	t, found := engine.ttlJanitor.GetTTL(string(key))
	return t, found, nil
}

func (engine *Engine) Delete(key []byte) {
	if err := engine.checkRateLimit(); err != nil {
		fmt.Println(err)
		return
	}
	if string(key) == token_bucket.InternalKey {
		return
	}

	if config.GetSettings().Debug {
		fmt.Printf("\nDeleting key %s\n", string(key))
	}
	seqId := engine.seqGen.Next()
	// wal
	engine.lsm.Put(key, nil, seqId, enums.OpTypeDel)
	engine.notifier.NotifyDelete(key)
}

func (engine *Engine) Snapshot(key []byte) {
	engine.lsm.Snapshot(key)
}

func (engine *Engine) RangeDelete(startKey []byte, endKey []byte) {
	seqId := engine.seqGen.Next()
	engine.lsm.Put(startKey, endKey, seqId, enums.OpTypeRangeDel)
	engine.notifier.NotifyDeleteRange(startKey, endKey)
}

func (engine *Engine) Close() error {
	if engine.inMemoryTTL {
		engine.ttlJanitor.Stop()
	}
	//wal finish write
	err := engine.lsm.Finish()
	if err != nil {
		return err
	}
	return nil
}

func (engine *Engine) ClearAll() error {
	if err := engine.lsm.ClearAll(); err != nil {
		return fmt.Errorf("clear-all: lsm clear failed: %w", err)
	}

	dataDir := config.GetSettings().SavePath
	manifestPath := filepath.Join(dataDir, "MANIFEST")

	if err := os.Remove(manifestPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clear-all: failed to remove manifest: %w", err)
	}
	if engine.inMemoryTTL {
		engine.ttlJanitor.ClearAll()
	}

	return nil
}

func (engine *Engine) Subscribe(lower, upper string, bufferSize int) *notifier.Listener {
	return engine.notifier.Subscribe([]byte(lower), []byte(upper), bufferSize)
}

func (engine *Engine) Unsubscribe(l *notifier.Listener) {
	engine.notifier.Unsubscribe(l)
}

// GetVersions returns all versions of key, newest first.
// Only meaningful for keys that have been snapshotted.
func (engine *Engine) GetVersions(key []byte) ([]string, error) {
	values, err := engine.lsm.GetVersions(key, 0)
	if err != nil {
		return nil, err
	}
	result := make([]string, len(values))
	for i, v := range values {
		result[i] = string(v)
	}
	return result, nil
}

// GetVersion returns the nth version of key (0 = current/newest).
func (engine *Engine) GetVersion(key []byte, version int) (string, bool, error) {
	values, err := engine.lsm.GetVersions(key, version+1)
	if err != nil {
		return "", false, err
	}
	if version >= len(values) {
		return "", false, nil
	}
	return string(values[version]), true, nil
}
