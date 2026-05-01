package core

import (
	"testing"
	"time"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
)

func newTestEngine(t *testing.T) *Engine {
	cfg := config.NewDefaultConfig()
	cfg.SavePath = t.TempDir()

	config.TESTSetSettings(cfg)

	engine, err := NewEngine() // assuming you have this
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	return engine
}

func TestPutGet(t *testing.T) {
	e := newTestEngine(t)

	key := []byte("key1")
	val := []byte("value1")

	e.Put(key, val)

	got, ok, err := e.Get(key)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if !ok {
		t.Fatalf("expected key to exist")
	}
	if string(got) != string(val) {
		t.Fatalf("expected %s, got %s", val, got)
	}
}

func TestOverwrite(t *testing.T) {
	e := newTestEngine(t)

	key := []byte("key1")

	e.Put(key, []byte("v1"))
	e.Put(key, []byte("v2"))

	got, ok, _ := e.Get(key)
	if !ok || string(got) != "v2" {
		t.Fatalf("overwrite failed, got %s", got)
	}
}

func TestDelete(t *testing.T) {
	e := newTestEngine(t)

	key := []byte("key1")
	e.Put(key, []byte("value"))
	e.Delete(key)

	_, ok, _ := e.Get(key)
	if ok {
		t.Fatalf("expected key to be deleted")
	}
}

func TestPutWithTTL(t *testing.T) {
	e := newTestEngine(t)

	key := []byte("ttlKey")
	e.PutWithTTL(key, []byte("value"), 1) // 1 second TTL

	time.Sleep(2 * time.Second)

	_, ok, _ := e.Get(key)
	if ok {
		t.Fatalf("expected key to expire")
	}
}

func TestGetTTL(t *testing.T) {
	e := newTestEngine(t)

	key := []byte("ttlKey")
	e.PutWithTTL(key, []byte("value"), 5)

	ttl, ok, err := e.GetTTL(key)
	if err != nil {
		t.Fatalf("GetTTL error: %v", err)
	}
	if !ok {
		t.Fatalf("expected key to exist")
	}
	if ttl <= 0 {
		t.Fatalf("expected positive TTL, got %d", ttl)
	}
}

func TestRangeScan(t *testing.T) {
	e := newTestEngine(t)

	e.Put([]byte("a"), []byte("1"))
	e.Put([]byte("b"), []byte("2"))
	e.Put([]byte("c"), []byte("3"))

	results, err := e.RangeScan("a", "c", 0, 10)
	if err != nil {
		t.Fatalf("RangeScan error: %v", err)
	}

	if len(results) == 0 {
		t.Fatalf("expected results")
	}
}

func TestPrefixScan(t *testing.T) {
	e := newTestEngine(t)

	e.Put([]byte("user:1"), []byte("A"))
	e.Put([]byte("user:2"), []byte("B"))
	e.Put([]byte("order:1"), []byte("C"))

	results, err := e.PrefixScan("user:", 0, 10)
	if err != nil {
		t.Fatalf("PrefixScan error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestClearAll(t *testing.T) {
	e := newTestEngine(t)

	e.Put([]byte("k1"), []byte("v1"))
	e.Put([]byte("k2"), []byte("v2"))

	err := e.ClearAll(false)
	if err != nil {
		t.Fatalf("ClearAll error: %v", err)
	}

	_, ok, _ := e.Get([]byte("k1"))
	if ok {
		t.Fatalf("expected data to be cleared")
	}
}

func TestClose(t *testing.T) {
	e := newTestEngine(t)

	err := e.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}
