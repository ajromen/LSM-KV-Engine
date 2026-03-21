package block

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
)

func TestReadWrite(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "data.bin")
	cfg := config.NewDefaultConfig()
	cfg.BlockManager.BlockCacheMaxBlocks = 10
	config.TESTSetSettings(cfg)

	bm := NewBlockManager(16)

	key := BlockKey{
		FilePath: filePath,
		Offset:   0,
	}

	data := []byte("0123456789abcdef")

	if err := bm.Write(key, data); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	read, err := bm.Read(key)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if string(read) != string(data) {
		t.Fatalf("expected %q, got %q", data, read)
	}
}

func TestCache(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "cache.bin")

	cfg := config.NewDefaultConfig()
	cfg.BlockManager.BlockCacheMaxBlocks = 1
	config.TESTSetSettings(cfg)
	bm := NewBlockManager(4)

	key := BlockKey{
		FilePath: filePath,
		Offset:   0,
	}

	data := []byte("abcd")

	if err := bm.Write(key, data); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	_, err := bm.Read(key)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if err := os.Remove(filePath); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	read, err := bm.Read(key)
	if err != nil {
		t.Fatalf("Read from cache failed: %v", err)
	}

	if string(read) != string(data) {
		t.Fatalf("expected %q, got %q", data, read)
	}
}

func TestWriteAt(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "writeat.bin")

	cfg := config.NewDefaultConfig()
	cfg.BlockManager.BlockCacheMaxBlocks = 10
	config.TESTSetSettings(cfg)
	bm := NewBlockManager(4)

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("OpenFile failed: %v", err)
	}
	defer f.Close()

	key := BlockKey{
		FilePath: filePath,
		Offset:   2,
	}

	data := []byte("abcd")

	if err := bm.WriteAt(f, key, data); err != nil {
		t.Fatalf("WriteAt failed: %v", err)
	}

	read, err := bm.Read(key)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if string(read) != "abcd" {
		t.Fatalf("expected 'abcd', got %q", read)
	}
}

func TestInvalidBlockSize(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "invalid.bin")
	cfg := config.NewDefaultConfig()
	cfg.BlockManager.BlockCacheMaxBlocks = 10
	config.TESTSetSettings(cfg)

	bm := NewBlockManager(8)

	key := BlockKey{
		FilePath: filePath,
		Offset:   0,
	}

	err := bm.Write(key, []byte("0123456"))
	if err == nil {
		t.Fatalf("expected error for invalid block size")
	}
}
