package backup

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
	"github.com/ajromen/LSM-KV-Engine/internal/sstable"
	"github.com/ajromen/LSM-KV-Engine/internal/wal"
)

func setupTest(t *testing.T) (string, string) {
	dataDir := t.TempDir()
	backupDir := t.TempDir()
	cfg := config.NewDefaultConfig()
	cfg.SavePath = dataDir
	cfg.Backup.SaveDirectory = backupDir
	config.TESTSetSettings(cfg)
	return dataDir, backupDir
}

func fakeManifest(t *testing.T, dataDir string, files []string) sstable.Manifest {
	t.Helper()
	var entries []sstable.SSTableManifest
	for i, name := range files {
		p := filepath.Join(dataDir, name)
		if err := os.WriteFile(p, []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, sstable.SSTableManifest{BaseFileName: p, Id: i, Layer: 0})
	}
	return sstable.Manifest{FileDir: dataDir, Layers: map[int][]sstable.SSTableManifest{0: entries}}
}

func fakeWALManifest(t *testing.T) wal.WALManifest {
	t.Helper()
	return wal.WALManifest{
		FileDir:      t.TempDir(),
		LowWatermark: 0,
		Segments:     make([]wal.WALManifestEntry, 0),
	}
}

func TestFullBackup(t *testing.T) {
	dataDir, _ := setupTest(t)
	manifest := fakeManifest(t, dataDir, []string{"L0_000000.sst"})
	walManifest := fakeWALManifest(t)
	restoreDir := t.TempDir()

	fb := NewFullBackup(nil)
	if err := fb.Backup(manifest, walManifest); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	info := fb.GetInfo()
	if len(info.Files) != 1 || info.Type != enums.FullBackup {
		t.Errorf("unexpected info: %+v", info)
	}
	if !fb.ContainsFile("L0_000000.sst") {
		t.Error("ContainsFile false for existing file")
	}

	if err := fb.Restore(restoreDir); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if _, err := os.Stat(filepath.Join(restoreDir, "L0_000000.sst")); err != nil {
		t.Error("restored file missing")
	}
	if _, err := os.Stat(filepath.Join(restoreDir, sstable.ManifestFileName)); err != nil {
		t.Error("restored manifest missing")
	}
}

func TestIncrementalBackup(t *testing.T) {
	dataDir, _ := setupTest(t)
	restoreDir := t.TempDir()
	walManifest := fakeWALManifest(t)

	fb := NewFullBackup(nil)
	if err := fb.Backup(fakeManifest(t, dataDir, []string{"L0_000000.sst"}), walManifest); err != nil {
		t.Fatalf("full Backup: %v", err)
	}

	var base IBackup = fb
	ib := NewIncrementalBackup(nil, &base)
	manifest2 := fakeManifest(t, dataDir, []string{"L0_000000.sst", "L0_000001.sst"})
	if err := ib.Backup(manifest2, walManifest); err != nil {
		t.Fatalf("incremental Backup: %v", err)
	}

	info := ib.GetInfo()
	if len(info.NewFiles) != 1 || info.NewFiles[0] != "L0_000001.sst" {
		t.Errorf("expected 1 new file L0_000001.sst, got %v", info.NewFiles)
	}
	if !ib.ContainsFile("L0_000000.sst") {
		t.Error("should contain base file")
	}

	if err := ib.Restore(restoreDir); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	for _, f := range []string{"L0_000000.sst", "L0_000001.sst", sstable.ManifestFileName} {
		if _, err := os.Stat(filepath.Join(restoreDir, f)); err != nil {
			t.Errorf("missing after restore: %s", f)
		}
	}
}

func TestIncrementalBackup_NoBase_ReturnsError(t *testing.T) {
	_, _ = setupTest(t)
	ib := NewIncrementalBackup(nil, nil)
	if err := ib.Restore(t.TempDir()); err == nil {
		t.Error("expected error with no base")
	}
}

func TestCheckpoint(t *testing.T) {
	dataDir, _ := setupTest(t)
	restoreDir := t.TempDir()
	manifest := fakeManifest(t, dataDir, []string{"L0_000000.sst"})
	walManifest := fakeWALManifest(t)

	cp := NewCheckpoint(nil)
	if err := cp.Backup(manifest, walManifest); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if cp.GetInfo().Type != enums.Checkpoint {
		t.Error("expected Checkpoint type")
	}

	if err := cp.Restore(restoreDir); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if err := os.WriteFile(filepath.Join(restoreDir, "L0_000000.sst"), []byte("modified"), 0644); err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(filepath.Join(cp.GetInfo().SaveDirectory, "L0_000000.sst"))
	if string(content) != "data" {
		t.Error("checkpoint file was modified through restore")
	}
}

func TestBackupManager(t *testing.T) {
	dataDir, _ := setupTest(t)
	manifest := fakeManifest(t, dataDir, []string{"L0_000000.sst"})
	walManifest := fakeWALManifest(t)

	bm, err := NewBackupManager()
	if err != nil {
		t.Fatalf("NewBackupManager: %v", err)
	}

	fullId, err := bm.CreateBackup(manifest, walManifest, enums.FullBackup)
	if err != nil {
		t.Fatalf("CreateBackup full: %v", err)
	}

	time.Sleep(time.Second)

	incrId, err := bm.CreateBackup(manifest, walManifest, enums.IncrementalBackup)
	if err != nil {
		t.Fatalf("CreateBackup incr: %v", err)
	}

	if (*bm.GetById(incrId)).GetInfo().BaseId != fullId {
		t.Error("incremental BaseId mismatch")
	}
	if err := bm.DeleteBackup(fullId); err == nil {
		t.Error("expected error deleting backup with dependents")
	}
	if err := bm.CascadeDelete(fullId); err != nil {
		t.Fatalf("CascadeDelete: %v", err)
	}
	if bm.GetById(fullId) != nil || bm.GetById(incrId) != nil {
		t.Error("expected both backups deleted after cascade")
	}
}

func TestBackupManager_PersistsAcrossRestart(t *testing.T) {
	dataDir, _ := setupTest(t)
	manifest := fakeManifest(t, dataDir, []string{"L0_000000.sst"})
	walManifest := fakeWALManifest(t)

	bm, _ := NewBackupManager()
	id, err := bm.CreateBackup(manifest, walManifest, enums.FullBackup)
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}

	bm2, err := NewBackupManager()
	if err != nil {
		t.Fatalf("restart: %v", err)
	}
	if bm2.GetById(id) == nil {
		t.Error("backup not persisted after restart")
	}
	if len((*bm2.GetById(id)).GetInfo().Files) != 1 {
		t.Error("files not persisted after restart")
	}
}
