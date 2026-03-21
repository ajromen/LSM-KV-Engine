package probabilistics

import (
	"testing"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
)

func seedOf(b byte, n int) []byte {
	if n <= 0 {
		n = 32
	}
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}

func TestNewSimHash_FromConfig(t *testing.T) {
	s := NewSimHash(config.SimHashConfig{Enabled: true})
	if s == nil {
		t.Fatal("expected non-nil SimHash")
	}
	seed := s.Seed()
	if len(seed) == 0 {
		t.Fatal("expected non-empty seed")
	}
	if fp, ok := s.Fingerprint(); ok || fp != 0 {
		t.Fatalf("expected empty fingerprint at start, got fp=%d ok=%v", fp, ok)
	}
}

func TestNewSimHashWithSeed_CopiesInputAndSeedGetterReturnsCopy(t *testing.T) {
	in := seedOf(0x2A, 32)
	s := NewSimHashWithSeed(in)

	// promena originalnog ulaza ne sme menjati interni seed
	in[0] = 0xFF
	got1 := s.Seed()
	if len(got1) != 32 {
		t.Fatalf("seed length mismatch: got=%d expected=32", len(got1))
	}
	if got1[0] != 0x2A {
		t.Fatalf("seed not copied correctly: got first byte=%x expected=2a", got1[0])
	}

	// promena vraćene kopije ne sme menjati interni seed
	got1[1] = 0xEE
	got2 := s.Seed()
	if got2[1] != 0x2A {
		t.Fatalf("Seed() should return copy; internal seed changed to %x", got2[1])
	}
}
