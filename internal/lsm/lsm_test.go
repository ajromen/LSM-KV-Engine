package lsm

import (
	"fmt"
	"os"
	"testing"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
	"github.com/ajromen/LSM-KV-Engine/internal/sequence"
)

func newSeq() *sequence.SequenceGenerator {
	return sequence.NewSequenceGenerator(0)
}

func newConfig(compaction enums.LSMCompaction) {
	cfg := config.NewDefaultConfig()
	cfg.SSTable.Format = enums.FormatSingleFile
	cfg.LSMTree.CompactionAlgorithm = compaction
	cfg.LSMTree.MinMergeThreshold = 4
	cfg.LSMTree.MaxHeight = 5
	cfg.LSMTree.LevelSizeMultiplier = 2 // L1MaxBytes = 512 * 2 = 1024B
	cfg.Memtable.MemtableMaxEntries = 5
	cfg.Memtable.MemtableMaxSizeBytes = 512
	config.TESTSetSettings(cfg)
}
func setupLSM(t *testing.T, compaction enums.LSMCompaction) (*LSM, string) {
	t.Helper()
	block.GetBlockCacheInstance().Clear()
	dir, err := os.MkdirTemp("", "lsm_test_*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	newConfig(compaction)
	lsm, err := NewLSM(dir, func() {})
	if err != nil {
		t.Fatalf("NewLSM: %v", err)
	}
	t.Cleanup(func() {
		if err := lsm.Finish(); err != nil {
			t.Logf("Finish: %v", err)
		}
		os.RemoveAll(dir)
	})
	return lsm, dir
}

func putN(t *testing.T, lsm *LSM, n int, seq *sequence.SequenceGenerator) {
	t.Helper()
	for i := 0; i < n; i++ {
		lsm.Put([]byte(fmt.Sprintf("key%05d", i)), []byte(fmt.Sprintf("val%05d", i)), seq.Next(), enums.OpTypePut)
	}
}

func putRange(t *testing.T, lsm *LSM, start, end int, valuePrefix string, seq *sequence.SequenceGenerator) {
	t.Helper()
	for i := start; i < end; i++ {
		lsm.Put([]byte(fmt.Sprintf("key%05d", i)), []byte(fmt.Sprintf("%s%05d", valuePrefix, i)), seq.Next(), enums.OpTypePut)
	}
}

func delRange(t *testing.T, lsm *LSM, start, end int, seq *sequence.SequenceGenerator) {
	t.Helper()
	for i := start; i < end; i++ {
		lsm.Put([]byte(fmt.Sprintf("key%05d", i)), nil, seq.Next(), enums.OpTypeDel)
	}
}

func assertGet(t *testing.T, lsm *LSM, key, expected string) {
	t.Helper()
	val, found, err := lsm.Get([]byte(key))
	if err != nil {
		t.Fatalf("Get(%s): %v", key, err)
	}
	if !found {
		t.Fatalf("Get(%s): not found, expected %q", key, expected)
	}
	if string(val) != expected {
		t.Fatalf("Get(%s): expected %q, got %q", key, expected, string(val))
	}
}

func assertNotFound(t *testing.T, lsm *LSM, key string) {
	t.Helper()
	_, found, err := lsm.Get([]byte(key))
	if err != nil {
		t.Fatalf("Get(%s): %v", key, err)
	}
	if found {
		t.Fatalf("Get(%s): expected not found", key)
	}
}

// ===== Basic =====

func runBasicTests(t *testing.T, compaction enums.LSMCompaction) {
	t.Run("PutGet", func(t *testing.T) {
		lsm, _ := setupLSM(t, compaction)
		seq := newSeq()
		lsm.Put([]byte("k"), []byte("v"), seq.Next(), enums.OpTypePut)
		assertGet(t, lsm, "k", "v")
	})

	t.Run("GetMissing", func(t *testing.T) {
		lsm, _ := setupLSM(t, compaction)
		assertNotFound(t, lsm, "missing")
	})

	t.Run("Overwrite", func(t *testing.T) {
		lsm, _ := setupLSM(t, compaction)
		seq := newSeq()
		lsm.Put([]byte("k"), []byte("v1"), seq.Next(), enums.OpTypePut)
		lsm.Put([]byte("k"), []byte("v2"), seq.Next(), enums.OpTypePut)
		assertGet(t, lsm, "k", "v2")
	})

	t.Run("Delete", func(t *testing.T) {
		lsm, _ := setupLSM(t, compaction)
		seq := newSeq()
		lsm.Put([]byte("k"), []byte("v"), seq.Next(), enums.OpTypePut)
		lsm.Put([]byte("k"), nil, seq.Next(), enums.OpTypeDel)
		assertNotFound(t, lsm, "k")
	})

	t.Run("DeleteNonExistent", func(t *testing.T) {
		lsm, _ := setupLSM(t, compaction)
		seq := newSeq()
		lsm.Put([]byte("ghost"), nil, seq.Next(), enums.OpTypeDel)
		assertNotFound(t, lsm, "ghost")
	})

	t.Run("FlushAndRead", func(t *testing.T) {
		lsm, _ := setupLSM(t, compaction)
		seq := newSeq()
		putN(t, lsm, 30, seq)
		for i := 0; i < 30; i++ {
			assertGet(t, lsm, fmt.Sprintf("key%05d", i), fmt.Sprintf("val%05d", i))
		}
	})

	t.Run("DeleteAfterFlush", func(t *testing.T) {
		lsm, _ := setupLSM(t, compaction)
		seq := newSeq()
		putN(t, lsm, 20, seq)
		lsm.Put([]byte("key00005"), nil, seq.Next(), enums.OpTypeDel)
		assertNotFound(t, lsm, "key00005")
		assertGet(t, lsm, "key00010", "val00010")
	})

	t.Run("OverwriteAfterFlush", func(t *testing.T) {
		lsm, _ := setupLSM(t, compaction)
		seq := newSeq()
		putN(t, lsm, 20, seq)
		lsm.Put([]byte("key00003"), []byte("new_value"), seq.Next(), enums.OpTypePut)
		assertGet(t, lsm, "key00003", "new_value")
	})

	t.Run("OverwriteAcrossMultipleFlushes", func(t *testing.T) {
		lsm, _ := setupLSM(t, compaction)
		seq := newSeq()
		putRange(t, lsm, 0, 20, "old", seq)
		putRange(t, lsm, 0, 20, "new", seq)
		for i := 0; i < 20; i++ {
			assertGet(t, lsm, fmt.Sprintf("key%05d", i), fmt.Sprintf("new%05d", i))
		}
	})
}

// ===== SizeTiered =====

func TestSizeTiered_Basic(t *testing.T) { runBasicTests(t, enums.SizeTieredCompaction) }

func TestSizeTiered_DataIntactAfterCompaction(t *testing.T) {
	lsm, _ := setupLSM(t, enums.SizeTieredCompaction)
	seq := newSeq()
	putN(t, lsm, 100, seq)
	for i := 0; i < 100; i++ {
		assertGet(t, lsm, fmt.Sprintf("key%05d", i), fmt.Sprintf("val%05d", i))
	}
}

func TestSizeTiered_LayersGrow(t *testing.T) {
	lsm, _ := setupLSM(t, enums.SizeTieredCompaction)
	seq := newSeq()
	putN(t, lsm, 500, seq)
	if len(lsm.sstableManager.Layers) < 2 {
		t.Error("expected at least 2 layers after heavy write load")
	}
}

// ===== Leveled =====

func TestLeveled_Basic(t *testing.T) { runBasicTests(t, enums.LeveledCompaction) }

func TestLeveled_DataIntactAfterCompaction(t *testing.T) {
	lsm, _ := setupLSM(t, enums.LeveledCompaction)
	seq := newSeq()
	putN(t, lsm, 100, seq)
	for i := 0; i < 100; i++ {
		assertGet(t, lsm, fmt.Sprintf("key%05d", i), fmt.Sprintf("val%05d", i))
	}
}

func TestLeveled_DeleteAfterCompaction(t *testing.T) {
	lsm, _ := setupLSM(t, enums.LeveledCompaction)
	seq := newSeq()
	putN(t, lsm, 100, seq)
	delRange(t, lsm, 0, 10, seq)
	for i := 0; i < 10; i++ {
		assertNotFound(t, lsm, fmt.Sprintf("key%05d", i))
	}
	for i := 10; i < 100; i++ {
		assertGet(t, lsm, fmt.Sprintf("key%05d", i), fmt.Sprintf("val%05d", i))
	}
}

func TestLeveled_OverlapResolved(t *testing.T) {
	lsm, _ := setupLSM(t, enums.LeveledCompaction)
	seq := newSeq()
	putRange(t, lsm, 0, 50, "old", seq)
	putRange(t, lsm, 0, 50, "new", seq)
	for i := 0; i < 50; i++ {
		assertGet(t, lsm, fmt.Sprintf("key%05d", i), fmt.Sprintf("new%05d", i))
	}
}

func TestLeveled_InterleavedWrites(t *testing.T) {
	lsm, _ := setupLSM(t, enums.LeveledCompaction)
	seq := newSeq()
	for i := 0; i < 60; i++ {
		prefix := "a"
		if i%2 != 0 {
			prefix = "b"
		}
		lsm.Put([]byte(fmt.Sprintf("key%05d", i)), []byte(fmt.Sprintf("%s%05d", prefix, i)), seq.Next(), enums.OpTypePut)
	}
	for i := 0; i < 60; i++ {
		prefix := "a"
		if i%2 != 0 {
			prefix = "b"
		}
		assertGet(t, lsm, fmt.Sprintf("key%05d", i), fmt.Sprintf("%s%05d", prefix, i))
	}
}

func TestLeveled_ReverseKeyOrder(t *testing.T) {
	lsm, _ := setupLSM(t, enums.LeveledCompaction)
	seq := newSeq()
	for i := 99; i >= 0; i-- {
		lsm.Put([]byte(fmt.Sprintf("key%05d", i)), []byte(fmt.Sprintf("val%05d", i)), seq.Next(), enums.OpTypePut)
	}
	for i := 0; i < 100; i++ {
		assertGet(t, lsm, fmt.Sprintf("key%05d", i), fmt.Sprintf("val%05d", i))
	}
}

func TestLeveled_DeleteThenReinsert(t *testing.T) {
	lsm, _ := setupLSM(t, enums.LeveledCompaction)
	seq := newSeq()
	putN(t, lsm, 30, seq)
	delRange(t, lsm, 0, 15, seq)
	putRange(t, lsm, 0, 15, "new", seq)
	for i := 0; i < 15; i++ {
		assertGet(t, lsm, fmt.Sprintf("key%05d", i), fmt.Sprintf("new%05d", i))
	}
	for i := 15; i < 30; i++ {
		assertGet(t, lsm, fmt.Sprintf("key%05d", i), fmt.Sprintf("val%05d", i))
	}
}

func TestLeveled_LayersGrowUnderLoad(t *testing.T) {
	lsm, _ := setupLSM(t, enums.LeveledCompaction)
	seq := newSeq()
	putN(t, lsm, 500, seq)
	t.Logf("Total layers: %d", len(lsm.sstableManager.Layers))
	if len(lsm.sstableManager.Layers) < 2 {
		t.Error("expected at least 2 layers after heavy write load")
	}
}

// ===== Persistence =====

func testPersistence(t *testing.T, compaction enums.LSMCompaction) {
	dir, err := os.MkdirTemp("", "lsm_persist_*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	newConfig(compaction)
	lsm1, err := NewLSM(dir, func() {})
	if err != nil {
		t.Fatalf("NewLSM: %v", err)
	}
	seq := newSeq()
	putN(t, lsm1, 50, seq)
	_ = lsm1.Finish()

	lsm2, err := NewLSM(dir, func() {})
	if err != nil {
		t.Fatalf("NewLSM reopen: %v", err)
	}
	defer lsm2.Finish()
	for i := 0; i < 50; i++ {
		assertGet(t, lsm2, fmt.Sprintf("key%05d", i), fmt.Sprintf("val%05d", i))
	}
}

func TestSizeTiered_Persistence(t *testing.T) { testPersistence(t, enums.SizeTieredCompaction) }
func TestLeveled_Persistence(t *testing.T)    { testPersistence(t, enums.LeveledCompaction) }
