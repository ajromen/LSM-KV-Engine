package core

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ajromen/LSM-KV-Engine/internal/backup"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
	"github.com/ajromen/LSM-KV-Engine/internal/lsm"
	"github.com/ajromen/LSM-KV-Engine/internal/merge"
	"github.com/ajromen/LSM-KV-Engine/internal/notifier"
	"github.com/ajromen/LSM-KV-Engine/internal/probabilistics"
	"github.com/ajromen/LSM-KV-Engine/internal/sequence"
	"github.com/ajromen/LSM-KV-Engine/internal/shared"
	"github.com/ajromen/LSM-KV-Engine/internal/token_bucket"
	"github.com/ajromen/LSM-KV-Engine/internal/ttl"
	"github.com/ajromen/LSM-KV-Engine/internal/wal"
)

var defaultSimHashSeed = []byte("lsm-kv-engine-simhash-seed-v1")

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

	newLsm, err := lsm.NewLSM(dataDir, engine.MemtableFlushed)
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
		case enums.OpTypeMerge:
			engine.lsm.Put(r.Key, r.Value, r.SeqId, enums.OpTypeMerge)
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
	if merge.IsProbKey(key) {
		return
	}

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
	if merge.IsProbKey(key) {
		return
	}
	seqId := engine.seqGen.Next()
	engine.wal.Put(key, value, seqId, enums.OpTypePut)
	if engine.inMemoryTTL {
		engine.ttlJanitor.AddTTL(shared.TTLEntry{ExpiresAt: time.Now().UnixMilli() + ttl, Key: key})
	}
	engine.lsm.PutWithTTL(key, value, seqId, enums.OpTypePut, ttl)
	engine.notifier.NotifyPut(key, value)
}

func (engine *Engine) Get(key []byte) ([]byte, bool, error) {
	if merge.IsProbKey(key) {
		return nil, false, nil
	}

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
	if merge.IsProbKey(key) {
		return
	}
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
	if merge.IsProbKey(startKey) || merge.IsProbKey(endKey) {
		return
	}
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

func (engine *Engine) MemtableFlushed() {
	err := engine.wal.MemtableFlushed(engine.seqGen.Current())
	if err != nil {
		panic(err)
	}
}

func (engine *Engine) ClearAll(forBackup bool) error {
	if err := engine.lsm.ClearAll(); err != nil {
		return fmt.Errorf("clear-all: lsm clear failed: %w", err)
	}

	if err := engine.wal.ClearAll(!forBackup); err != nil {
		return fmt.Errorf("clear-all: wal clear failed: %w", err)
	}

	if err := clearDataDir(config.GetSettings().SavePath); err != nil {
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

// ---- BloomFilter ----

func (engine *Engine) BFCreate(name string, expectedElements uint, falsePositiveRate float64) error {
	key := merge.ProbKey(merge.TypeBloom, name)

	exists, err := engine.probExists(key)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("BloomFilter '%s' already exists", name)
	}

	bf := probabilistics.NewBloomFilterWithParams(expectedElements, falsePositiveRate, nil)
	data, err := bf.ToBytes()
	if err != nil {
		return err
	}

	engine.probPut(key, merge.WrapBaseState(data), enums.OpTypeMerge)
	return nil
}

func (engine *Engine) BFAdd(name string, element []byte) error {
	key := merge.ProbKey(merge.TypeBloom, name)

	if err := engine.ensureProbExists(key, "BloomFilter", name); err != nil {
		return err
	}

	operand := merge.WrapOperand(merge.OpAdd, element)
	engine.probPut(key, operand, enums.OpTypeMerge)
	return nil
}

func (engine *Engine) BFContains(name string, element []byte) (bool, error) {
	key := merge.ProbKey(merge.TypeBloom, name)

	state, found, err := engine.probGet(key)
	if err != nil {
		return false, err
	}
	if !found {
		return false, fmt.Errorf("BloomFilter '%s' does not exist", name)
	}

	bf := &probabilistics.BloomFilter{}
	if err := bf.FromBytes(state); err != nil {
		return false, err
	}

	return bf.MightContain(element), nil
}

func (engine *Engine) BFDelete(name string) {

	engine.probDelete(merge.ProbKey(merge.TypeBloom, name))

}

// ---- CountMinSketch ----

func (engine *Engine) CMSCreate(name string) error {
	key := merge.ProbKey(merge.TypeCMS, name)

	exists, err := engine.probExists(key)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("CountMinSketch '%s' already exists", name)
	}

	cfg := config.GetSettings().ProbabilisticType.CountMinSketch
	cms := probabilistics.NewCountMinSketch(cfg)

	var buf bytes.Buffer
	if _, err := cms.WriteTo(&buf); err != nil {
		return err
	}

	engine.probPut(key, merge.WrapBaseState(buf.Bytes()), enums.OpTypeMerge)
	return nil
}

func (engine *Engine) CMSAdd(name string, event []byte) error {
	key := merge.ProbKey(merge.TypeCMS, name)

	if err := engine.ensureProbExists(key, "CountMinSketch", name); err != nil {
		return err
	}

	operand := merge.WrapOperand(merge.OpAdd, event)
	engine.probPut(key, operand, enums.OpTypeMerge)
	return nil
}

func (engine *Engine) CMSFrequency(name string, event []byte) (uint, error) {
	key := merge.ProbKey(merge.TypeCMS, name)

	state, found, err := engine.probGet(key)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, fmt.Errorf("CountMinSketch '%s' does not exist", name)
	}

	cms := &probabilistics.CountMinSketch{}
	if _, err := cms.ReadFrom(bytes.NewReader(state)); err != nil {
		return 0, err
	}

	return cms.Estimate(event), nil
}

func (engine *Engine) CMSDelete(name string) {

	engine.probDelete(merge.ProbKey(merge.TypeCMS, name))

}

// ---- HyperLogLog ----

func (engine *Engine) HLLCreate(name string) error {
	key := merge.ProbKey(merge.TypeHLL, name)

	exists, err := engine.probExists(key)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("HyperLogLog '%s' already exists", name)
	}

	hll := probabilistics.NewHyperLogLog()
	data, err := hll.ToBytes()
	if err != nil {
		return err
	}

	engine.probPut(key, merge.WrapBaseState(data), enums.OpTypeMerge)
	return nil
}

func (engine *Engine) HLLAdd(name string, element []byte) error {
	key := merge.ProbKey(merge.TypeHLL, name)

	if err := engine.ensureProbExists(key, "HyperLogLog", name); err != nil {
		return err
	}

	operand := merge.WrapOperand(merge.OpAdd, element)
	engine.probPut(key, operand, enums.OpTypeMerge)
	return nil
}

func (engine *Engine) HLLCardinality(name string) (uint64, error) {
	key := merge.ProbKey(merge.TypeHLL, name)

	state, found, err := engine.probGet(key)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, fmt.Errorf("HyperLogLog '%s' does not exist", name)
	}

	hll := &probabilistics.HyperLogLog{}
	if err := hll.FromBytes(state); err != nil {
		return 0, err
	}

	return hll.Count(), nil
}

func (engine *Engine) HLLDelete(name string) {

	engine.probDelete(merge.ProbKey(merge.TypeHLL, name))

}

// ---- SimHash ----
// SimHash does NOT use merge operators, each call stores/replaces the fingerprint.

func (engine *Engine) SimHashStore(name string, text string) error {
	key := merge.ProbKey(merge.TypeSimHash, name)
	sh := probabilistics.NewSimHashWithSeed(defaultSimHashSeed)
	sh.HashText(text)
	data, err := sh.ToBytes()
	if err != nil {
		return err
	}
	// SimHash uses normal Upsert, only latest fingerprint is relevant
	engine.probPut(key, merge.WrapBaseState(data), enums.OpTypePut)
	return nil
}

func (engine *Engine) SimHashDistance(name1, name2 string) (uint8, error) {
	get := func(name string) (*probabilistics.SimHash, error) {
		key := merge.ProbKey(merge.TypeSimHash, name)
		v, found, err := engine.lsm.Get(key)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, fmt.Errorf("SimHash '%s' not found", name)
		}
		if len(v) == 0 || v[0] != merge.BaseStateMarker {
			return nil, fmt.Errorf("invalid SimHash state for '%s'", name)
		}
		sh := &probabilistics.SimHash{}
		if err := sh.FromBytes(v[1:]); err != nil {
			return nil, err
		}
		return sh, nil
	}
	sh1, err := get(name1)
	if err != nil {
		return 0, err
	}
	sh2, err := get(name2)
	if err != nil {
		return 0, err
	}
	return sh1.DistanceTo(sh2)
}

func (engine *Engine) SimHashDelete(name string) {

	engine.probDelete(merge.ProbKey(merge.TypeSimHash, name))

}

// ---- internal helpers ----

func (engine *Engine) probPut(key []byte, value []byte, opType enums.OpType) {

	seqId := engine.seqGen.Next()
	engine.wal.Put(key, value, seqId, opType)
	engine.lsm.Put(key, value, seqId, opType)

}

func (engine *Engine) probDelete(key []byte) {

	seqId := engine.seqGen.Next()
	engine.wal.Put(key, nil, seqId, enums.OpTypeDel)
	engine.lsm.Put(key, nil, seqId, enums.OpTypeDel)

}

func (engine *Engine) probGet(key []byte) ([]byte, bool, error) {
	// First check the current visible state of the key.
	// This makes tombstones work correctly:
	// create -> add -> delete -> read should return not found.
	_, found, err := engine.lsm.Get(key)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}

	return engine.lsm.GetMerged(key)
}

func (engine *Engine) probExists(key []byte) (bool, error) {
	_, found, err := engine.probGet(key)
	if err != nil {
		return false, err
	}
	return found, nil
}

func (engine *Engine) ensureProbExists(key []byte, typeName string, name string) error {
	exists, err := engine.probExists(key)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%s '%s' does not exist", typeName, name)
	}
	return nil
}
