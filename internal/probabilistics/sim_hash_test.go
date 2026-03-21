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

	in[0] = 0xFF
	got1 := s.Seed()
	if len(got1) != 32 {
		t.Fatalf("seed length mismatch: got=%d expected=32", len(got1))
	}
	if got1[0] != 0x2A {
		t.Fatalf("seed not copied correctly: got first byte=%x expected=2a", got1[0])
	}

	got1[1] = 0xEE
	got2 := s.Seed()
	if got2[1] != 0x2A {
		t.Fatalf("Seed() should return copy; internal seed changed to %x", got2[1])
	}
}

func TestHashText_DeterministicForSameSeed(t *testing.T) {
	seed := seedOf(0x11, 32)
	text := "This is a SimHash test."

	s1 := NewSimHashWithSeed(seed)
	s2 := NewSimHashWithSeed(seed)

	fp1 := s1.HashText(text)
	fp2 := s2.HashText(text)

	if fp1 != fp2 {
		t.Fatalf("same seed + same text must produce same fingerprint: %d vs %d", fp1, fp2)
	}

	fp3 := ComputeSimHash(text, seed)
	if fp1 != fp3 {
		t.Fatalf("ComputeSimHash mismatch: %d vs %d", fp1, fp3)
	}

	if got, ok := s1.Fingerprint(); !ok || got != fp1 {
		t.Fatalf("Fingerprint() mismatch: got=%d ok=%v expected=%d", got, ok, fp1)
	}
}

func TestHashText_NormalizationCaseAndPunctuation(t *testing.T) {
	seed := seedOf(0x22, 32)

	a := "Hello, WORLD!! 123"
	b := "hello world 123"

	fpA := ComputeSimHash(a, seed)
	fpB := ComputeSimHash(b, seed)

	if fpA != fpB {
		t.Fatalf("expected same fingerprint after normalization: %d vs %d", fpA, fpB)
	}
}

func TestHashBytes_EqualsHashText(t *testing.T) {
	seed := seedOf(0x33, 32)
	text := "bytes and text should match"

	s1 := NewSimHashWithSeed(seed)
	s2 := NewSimHashWithSeed(seed)

	fp1 := s1.HashText(text)
	fp2 := s2.HashBytes([]byte(text))

	if fp1 != fp2 {
		t.Fatalf("HashText and HashBytes mismatch: %d vs %d", fp1, fp2)
	}
}

func TestDistanceMethods(t *testing.T) {
	seed := seedOf(0x44, 32)

	a := NewSimHashWithSeed(seed)
	b := NewSimHashWithSeed(seed)
	c := NewSimHashWithSeed(seed)

	fpA := a.HashText("same text for both")
	fpB := b.HashText("same text for both")
	fpC := c.HashText("completely different tokens 999 xyz")

	// isto -> distance 0
	dAB, err := a.DistanceTo(b)
	if err != nil {
		t.Fatalf("DistanceTo unexpected error: %v", err)
	}
	if dAB != 0 {
		t.Fatalf("expected distance 0 for identical fingerprints, got=%d", dAB)
	}

	// do svog fingerprint-a -> 0
	dA, err := a.DistanceToFingerprint(fpA)
	if err != nil {
		t.Fatalf("DistanceToFingerprint unexpected error: %v", err)
	}
	if dA != 0 {
		t.Fatalf("expected distance 0 to same fingerprint, got=%d", dA)
	}

	// različit tekst -> očekujemo > 0
	dAC, err := a.DistanceTo(c)
	if err != nil {
		t.Fatalf("DistanceTo unexpected error: %v", err)
	}
	if dAC == 0 && fpA != fpC {
		t.Fatalf("expected non-zero distance for different fingerprints")
	}
}
