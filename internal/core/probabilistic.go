package core

import (
	"bytes"
	"fmt"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
	"github.com/ajromen/LSM-KV-Engine/internal/merge"
	"github.com/ajromen/LSM-KV-Engine/internal/probabilistics"
)

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

	engine.lsm.SnapshotProbKey(key)
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

	engine.lsm.SnapshotProbKey(key)
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

	engine.lsm.SnapshotProbKey(key)
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
