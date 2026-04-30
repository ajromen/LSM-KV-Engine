package core

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ajromen/LSM-KV-Engine/internal/backup"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
	"github.com/ajromen/LSM-KV-Engine/internal/lsm"
	"github.com/ajromen/LSM-KV-Engine/internal/notifier"
	"github.com/ajromen/LSM-KV-Engine/internal/sequence"
	"github.com/ajromen/LSM-KV-Engine/internal/shared"
	"github.com/ajromen/LSM-KV-Engine/internal/token_bucket"
	"github.com/ajromen/LSM-KV-Engine/internal/ttl"
	"github.com/ajromen/LSM-KV-Engine/internal/wal"
)

func NewEngine() (*Engine, error) {
	dataDir := config.GetSettings().SavePath
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	backupManager, err := backup.NewBackupManager()
	if err != nil {
		return nil, err
	}

	engine := &Engine{
		inMemoryTTL:   config.GetSettings().TTL.InMemoryTTL,
		notifier:      notifier.NewNotifier(),
		backupManager: backupManager,
	}

	if err := engine.initializeComponents(); err != nil {
		return nil, err
	}

	return engine, nil
}

func (engine *Engine) initializeComponents() error {
	dataDir := config.GetSettings().SavePath

	newLsm, err := lsm.NewLSM(dataDir)
	if err != nil {
		return err
	}
	engine.lsm = newLsm

	engine.wal, err = wal.OpenWAL()
	if err != nil {
		return err
	}

	err = engine.recover()
	if err != nil {
		return err
	}

	cfg := config.GetSettings().TokenBucket
	if cfg.MaxTokens > 0 {
		engine.tokenBucket = nil
		existing, found, err := engine.lsm.Get([]byte(token_bucket.InternalKey))
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
		heap, err := engine.lsm.GetAllTTLFomSST()
		if err != nil {
			return err
		}
		engine.ttlJanitor.Init(heap)
		go engine.ttlJanitor.Run()
	}

	return nil
}

func (engine *Engine) reinitialize() error {
	if engine.inMemoryTTL {
		engine.ttlJanitor.Stop()
	}
	return engine.initializeComponents()
}

// check manifest
// check wal
func (engine *Engine) recover() error {
	var maxSeq uint64
	maxSeq = engine.lsm.GetMaxSeqId()

	records, err := engine.wal.Recover()
	if err != nil {
		return err
	}

	for _, r := range records {
		switch r.OpType {
		case enums.OpTypeDel:
			engine.lsm.Put(r.Key, nil, r.SeqId, enums.OpTypeDel)
		case enums.OpTypePut:
			engine.lsm.Put(r.Key, r.Value, r.SeqId, enums.OpTypePut)
		case enums.OpTypeRangeDel:
			engine.lsm.Put(r.Key, r.Value, r.SeqId, enums.OpTypeRangeDel)
		default:
			return fmt.Errorf("unknown op type: %v", r.OpType)
		}
		if r.SeqId > maxSeq {
			maxSeq = r.SeqId
		}
	}
	if config.GetSettings().Debug {
		fmt.Printf("Recovered %d records from WAL\n", len(records))
	}

	engine.seqGen = sequence.NewSequenceGenerator(maxSeq)
	return nil
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
	engine.wal.Put(key, value, seqId, enums.OpTypePut)
	engine.lsm.Put(key, value, seqId, enums.OpTypePut)
	engine.notifier.NotifyPut(key, value)
}

func (engine *Engine) PutWithTTL(key []byte, value []byte, ttl int64) {
	seqId := engine.seqGen.Next()
	engine.wal.Put(key, value, seqId, enums.OpTypePut)
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
	value, found, err := engine.lsm.GetTTL(key)
	return value, found, err
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
	engine.wal.Put(key, nil, seqId, enums.OpTypeDel)
	engine.lsm.Put(key, nil, seqId, enums.OpTypeDel)
	engine.notifier.NotifyDelete(key)
}

func (engine *Engine) RangeDelete(startKey []byte, endKey []byte) {
	seqId := engine.seqGen.Next()
	engine.wal.Put(startKey, endKey, seqId, enums.OpTypeRangeDel)
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

	err := clearDataDir(config.GetSettings().SavePath)
	if err != nil {
		return err
	}

	if engine.inMemoryTTL {
		engine.ttlJanitor.ClearAll()
	}

	return nil
}

func clearDataDir(dataDir string) error {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if err := os.Remove(filepath.Join(dataDir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func (engine *Engine) Subscribe(lower, upper string, bufferSize int) *notifier.Listener {
	return engine.notifier.Subscribe([]byte(lower), []byte(upper), bufferSize)
}

func (engine *Engine) Unsubscribe(l *notifier.Listener) {
	engine.notifier.Unsubscribe(l)
}
