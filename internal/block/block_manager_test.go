package block

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadWrite(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "data.bin")

	bm := NewBlockManager(16, 10)

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

	bm := NewBlockManager(4, 1)

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

	bm := NewBlockManager(4, 10)

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

	bm := NewBlockManager(8, 10)

	key := BlockKey{
		FilePath: filePath,
		Offset:   0,
	}

	err := bm.Write(key, []byte("0123456"))
	if err == nil {
		t.Fatalf("expected error for invalid block size")
	}
}
