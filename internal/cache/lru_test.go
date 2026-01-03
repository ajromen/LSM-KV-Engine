package cache

import (
	"testing"
)

func TestGetMissingKey(t *testing.T) {
	lru := NewLRU[string, string](2)

	if val, ok := lru.Get("a"); ok {
		t.Fatalf("expected not found, got %s", val)
	}
}

func TestPutAndGet(t *testing.T) {
	lru := NewLRU[string, string](2)

	lru.Put("a", "jedan")

	val, ok := lru.Get("a")
	if !ok {
		t.Fatal("expected found, got not found")
	}

	if val != "jedan" {
		t.Fatalf("expected 'jedan', got %s", val)
	}
}

func TestEviction(t *testing.T) {
	lru := NewLRU[string, string](2)

	lru.Put("a", "jedan")
	lru.Put("b", "dva")
	lru.Put("c", "tri")

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
	lru := NewLRU[string, string](2)

	lru.Put("a", "jedan")
	lru.Put("b", "dva")

	// refresh 'a'
	lru.Get("a")

	// "b" ispada
	lru.Put("c", "tri")

	if _, ok := lru.Get("b"); ok {
		t.Fatal("expected 'b' to be evicted")
	}

	if _, ok := lru.Get("a"); !ok {
		t.Fatal("expected 'a' to exist")
	}
}

func TestUpdateExistingKey(t *testing.T) {
	lru := NewLRU[string, string](2)

	lru.Put("a", "jedan")
	lru.Put("b", "dva")

	lru.Put("a", "novo")

	val, ok := lru.Get("a")
	if !ok {
		t.Fatal("expected 'a' to exist")
	}

	if val != "novo" {
		t.Fatalf("expected updated value, got %s", val)
	}

	lru.Put("c", "tri")

	if _, ok := lru.Get("b"); ok {
		t.Fatal("expected 'b' to be evicted")
	}
}
