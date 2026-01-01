package cache

import (
	"bytes"
	"testing"
)

func TestGetMissingKey(t *testing.T) {
	lru := NewLRU(2)

	if val, ok := lru.Get("a"); ok {
		t.Fatalf("expected not found, got %s", val)
	}
}

func TestPutAndGet(t *testing.T) {
	lru := NewLRU(2)

	lru.Put("a", []byte("jedan"))

	val, ok := lru.Get("a")
	if !ok {
		t.Fatal("expected found, got not found")
	}

	if !bytes.Equal(val.([]byte), []byte("jedan")) {
		t.Fatalf("expected 'jedan', got %s", val)
	}
}

func TestEviction(t *testing.T) {
	lru := NewLRU(2)

	lru.Put("a", []byte("jedan"))
	lru.Put("b", []byte("dva"))
	lru.Put("c", []byte("tri"))

	if _, ok := lru.Get("a"); ok {
		t.Fatal("expected 'a' to be evicted")
	}

	if _, ok := lru.Get("b"); !ok {
		t.Fatal("expected 'b' to exist")
	}

	if _, ok := lru.Get("c"); !ok {
		t.Fatal("expected 'c' to exist")
	}
}

func TestRefresh(t *testing.T) {
	lru := NewLRU(2)

	lru.Put("a", []byte("jedan"))
	lru.Put("b", []byte("dva"))

	// refresh 'a'
	lru.Get("a")

	// "b" ispada
	lru.Put("c", []byte("tri"))

	if _, ok := lru.Get("b"); ok {
		t.Fatal("expected 'b' to be evicted")
	}

	if _, ok := lru.Get("a"); !ok {
		t.Fatal("expected 'a' to exist")
	}
}

func TestUpdateExistingKey(t *testing.T) {
	lru := NewLRU(2)

	lru.Put("a", []byte("jedan"))
	lru.Put("b", []byte("dva"))

	lru.Put("a", []byte("novo"))

	val, ok := lru.Get("a")
	if !ok {
		t.Fatal("expected 'a' to exist")
	}

	if !bytes.Equal(val.([]byte), []byte("novo")) {
		t.Fatalf("expected updated value, got %s", val)
	}

	lru.Put("c", []byte("tri"))

	if _, ok := lru.Get("b"); ok {
		t.Fatal("expected 'b' to be evicted")
	}
}
