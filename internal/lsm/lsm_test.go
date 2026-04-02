package lsm

import (
	"fmt"
	"os"
	"testing"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
	"github.com/ajromen/LSM-KV-Engine/internal/sequence"
)

func newSeq() *sequence.SequenceGenerator {
	return sequence.NewSequenceGenerator(0)
}

func newConfig(compaction enums.LSMCompaction) {
	cfg := config.NewDefaultConfig()
	cfg.LSMTree.CompactionAlgorithm = compaction
	cfg.LSMTree.MinMergeThreshold = 4
	cfg.LSMTree.LevelSizeMultiplier = 10
	cfg.Memtable.MemtableMaxEntries = 5
	cfg.Memtable.MemtableMaxSizeBytes = 1024 * 4
	config.TESTSetSettings(cfg)
}

func setupLSM(t *testing.T, compaction enums.LSMCompaction) (*LSM, string) {
	t.Helper()
	dir, err := os.MkdirTemp("", "lsm_test_*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	newConfig(compaction)
	lsm, err := NewLSM(dir)
	if err != nil {
		t.Fatalf("NewLSM: %v", err)
	}
	t.Cleanup(func() {
		err := lsm.Finish()
		if err != nil {
			return
		}
		teardown(dir)
	})
	return lsm, dir
}

func teardown(dir string) { os.RemoveAll(dir) }

func putN(t *testing.T, lsm *LSM, n int, seq *sequence.SequenceGenerator) {
	t.Helper()
	for i := 0; i < n; i++ {
		lsm.Put([]byte(fmt.Sprintf("key%05d", i)), []byte(fmt.Sprintf("val%05d", i)), seq.Next(), enums.OpTypePut)
	}
}

func assertGet(t *testing.T, lsm *LSM, key, expected string) {
	t.Helper()
	val, found, err := lsm.Get([]byte(key))
	if err != nil {
		t.Fatalf("Get(%s): %v", key, err)
	}
	if !found {
		t.Fatalf("Get(%s): not found", key)
	}
	if string(val) != expected {
		t.Fatalf("Get(%s): expected %q, got %q", key, expected, val)
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
}

func TestSizeTiered_Basic(t *testing.T) {
	runBasicTests(t, enums.SizeTieredCompaction)
}

/*
func TestSizeTiered_CompactionReducesL0(t *testing.T) {
	lsm, _ := setupLSM(t, enums.SizeTieredCompaction)
	seq := newSeq()
	putN(t, lsm, 200, seq)
	l0 := lsm.sstableManager.Layers[0].Length()
	t.Logf("L0 count after 200 puts: %d", l0)
	threshold := config.GetSettings().LSMTree.MinMergeThreshold
	if l0 >= threshold {
		t.Errorf("L0 should have been compacted, got %d (threshold %d)", l0, threshold)
	}
}
*/

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
	t.Logf("Total layers: %d", len(lsm.sstableManager.Layers))
	if len(lsm.sstableManager.Layers) < 2 {
		t.Error("expected at least 2 layers after heavy write load")
	}
}

func TestLeveled_Basic(t *testing.T) {
	runBasicTests(t, enums.LeveledCompaction)
}

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
	for i := 0; i < 10; i++ {
		lsm.Put([]byte(fmt.Sprintf("key%05d", i)), nil, seq.Next(), enums.OpTypeDel)
	}
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
	for i := 0; i < 50; i++ {
		lsm.Put([]byte(fmt.Sprintf("key%05d", i)), []byte("old"), seq.Next(), enums.OpTypePut)
	}
	for i := 0; i < 50; i++ {
		lsm.Put([]byte(fmt.Sprintf("key%05d", i)), []byte("new"), seq.Next(), enums.OpTypePut)
	}
	for i := 0; i < 50; i++ {
		assertGet(t, lsm, fmt.Sprintf("key%05d", i), "new")
	}
}

func testPersistence(t *testing.T, compaction enums.LSMCompaction) {
	dir, err := os.MkdirTemp("", "lsm_persist_*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})

	newConfig(compaction)

	lsm1, err := NewLSM(dir)
	if err != nil {
		t.Fatalf("NewLSM: %v", err)
	}
	seq := newSeq()
	putN(t, lsm1, 50, seq)
	_ = lsm1.Finish()

	lsm2, err := NewLSM(dir)
	if err != nil {
		t.Fatalf("NewLSM reopen: %v", err)
	}
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("key%05d", i)
		val, found, err := lsm2.Get([]byte(key))
		if err != nil {
			t.Fatalf("Get(%s): %v", key, err)
		}
		if found && string(val) != fmt.Sprintf("val%05d", i) {
			t.Fatalf("key %s: expected val%05d, got %s", key, i, val)
		}
	}
}

func TestSizeTiered_Persistence(t *testing.T) { testPersistence(t, enums.SizeTieredCompaction) }
func TestLeveled_Persistence(t *testing.T)    { testPersistence(t, enums.LeveledCompaction) }
