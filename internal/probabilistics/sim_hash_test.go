package probabilistics

import (
	"bytes"
	"testing"
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
	s := NewSimHash()
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

	_ = b.HashText("same text for both")
	fpA := a.HashText("same text for both")
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

func TestDistanceMethods_MissingFingerprintErrors(t *testing.T) {
	seed := seedOf(0x55, 32)

	a := NewSimHashWithSeed(seed)
	b := NewSimHashWithSeed(seed)

	if _, err := a.DistanceTo(b); err == nil {
		t.Fatal("expected error when both fingerprints are missing")
	}

	a.HashText("only A has fingerprint")
	if _, err := a.DistanceTo(b); err == nil {
		t.Fatal("expected error when one fingerprint is missing")
	}

	c := NewSimHashWithSeed(seed)
	if _, err := c.DistanceToFingerprint(123); err == nil {
		t.Fatal("expected error for DistanceToFingerprint when fingerprint missing")
	}
}

func TestSetFingerprint_AndHexHelpers(t *testing.T) {
	s := NewSimHashWithSeed(seedOf(0x66, 32))

	const fp uint64 = 0x0102030405060708
	s.SetFingerprint(fp)

	got, ok := s.Fingerprint()
	if !ok || got != fp {
		t.Fatalf("SetFingerprint failed: got=%d ok=%v expected=%d", got, ok, fp)
	}

	hexStr, ok := s.FingerprintHex()
	if !ok {
		t.Fatal("expected FingerprintHex ok=true")
	}
	if hexStr != "0102030405060708" {
		t.Fatalf("FingerprintHex mismatch: got=%q", hexStr)
	}

	// set preko hex-a sa prefix-om
	if err := s.SetFingerprintHex("0x1122334455667788"); err != nil {
		t.Fatalf("SetFingerprintHex failed: %v", err)
	}
	got2, ok := s.Fingerprint()
	if !ok || got2 != 0x1122334455667788 {
		t.Fatalf("SetFingerprintHex mismatch: got=%x ok=%v", got2, ok)
	}
}

func TestHammingDistance64_KnownValues(t *testing.T) {
	// 1010 xor 0011 = 1001 -> 2 bita
	var a uint64 = 0b1010
	var b uint64 = 0b0011

	d := HammingDistance64(a, b)
	if d != 2 {
		t.Fatalf("expected HammingDistance64=2, got=%d", d)
	}
}

func TestHammingDistanceHex(t *testing.T) {
	d, err := HammingDistanceHex("0000000000000000", "ffffffffffffffff")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != 64 {
		t.Fatalf("expected distance=64, got=%d", d)
	}

	d2, err := HammingDistanceHex("0x000000000000000f", "0000000000000000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d2 != 4 {
		t.Fatalf("expected distance=4, got=%d", d2)
	}
}

func TestHammingDistanceHex_Invalid(t *testing.T) {
	if _, err := HammingDistanceHex("1234", "0000000000000000"); err == nil {
		t.Fatal("expected error for invalid hex length")
	}
	if _, err := HammingDistanceHex("zzzzzzzzzzzzzzzz", "0000000000000000"); err == nil {
		t.Fatal("expected error for invalid hex chars")
	}
}

func TestToBytesFromBytes_RoundTripWithoutFingerprint(t *testing.T) {
	s := NewSimHashWithSeed(seedOf(0x77, 32))

	b, err := s.ToBytes()
	if err != nil {
		t.Fatalf("ToBytes failed: %v", err)
	}
	if !IsSimHashBytes(b) {
		t.Fatal("expected IsSimHashBytes=true for serialized state")
	}

	var out SimHash
	if err := out.FromBytes(b); err != nil {
		t.Fatalf("FromBytes failed: %v", err)
	}

	if got, ok := out.Fingerprint(); ok || got != 0 {
		t.Fatalf("expected no fingerprint after roundtrip, got=%d ok=%v", got, ok)
	}
	if !bytes.Equal(s.Seed(), out.Seed()) {
		t.Fatal("seed mismatch after roundtrip")
	}
}

func TestToBytesFromBytes_RoundTripWithFingerprint(t *testing.T) {
	s := NewSimHashWithSeed(seedOf(0x88, 32))
	fp := s.HashText("serialized fingerprint test text")

	b, err := s.ToBytes()
	if err != nil {
		t.Fatalf("ToBytes failed: %v", err)
	}

	var out SimHash
	if err := out.FromBytes(b); err != nil {
		t.Fatalf("FromBytes failed: %v", err)
	}

	got, ok := out.Fingerprint()
	if !ok {
		t.Fatal("expected fingerprint present after roundtrip")
	}
	if got != fp {
		t.Fatalf("fingerprint mismatch after roundtrip: got=%d expected=%d", got, fp)
	}
	if !bytes.Equal(s.Seed(), out.Seed()) {
		t.Fatal("seed mismatch after roundtrip")
	}
}

func TestFromBytes_InvalidMagic(t *testing.T) {
	var s SimHash
	if err := s.FromBytes([]byte("BAD!")); err == nil {
		t.Fatal("expected error for invalid magic")
	}
}

func TestClear(t *testing.T) {
	s := NewSimHashWithSeed(seedOf(0x99, 32))
	s.HashText("some text")
	if _, ok := s.Fingerprint(); !ok {
		t.Fatal("expected fingerprint before Clear")
	}

	s.Clear()
	if fp, ok := s.Fingerprint(); ok || fp != 0 {
		t.Fatalf("expected cleared fingerprint, got=%d ok=%v", fp, ok)
	}
}

func TestIsSimHashBytes(t *testing.T) {
	if IsSimHashBytes(nil) {
		t.Fatal("nil should not be simhash bytes")
	}
	if IsSimHashBytes([]byte("SMH")) {
		t.Fatal("short bytes should not be simhash bytes")
	}
	if IsSimHashBytes([]byte("XXXX")) {
		t.Fatal("wrong magic should not be simhash bytes")
	}

	s := NewSimHashWithSeed(seedOf(0xAA, 32))
	b, err := s.ToBytes()
	if err != nil {
		t.Fatalf("ToBytes failed: %v", err)
	}
	if !IsSimHashBytes(b) {
		t.Fatal("expected true for valid serialized simhash")
	}
}

func TestTokenizeAndCount(t *testing.T) {
	in := "Go, go! gO 123 123 x-y"
	got := tokenizeAndCount(in)

	if got["go"] != 3 {
		t.Fatalf("expected go=3, got=%d", got["go"])
	}
	if got["123"] != 2 {
		t.Fatalf("expected 123=2, got=%d", got["123"])
	}
	if got["x"] != 1 || got["y"] != 1 {
		t.Fatalf("expected x=1 and y=1, got x=%d y=%d", got["x"], got["y"])
	}
}

func TestParseHexFingerprint(t *testing.T) {
	v, err := parseHexFingerprint("0x1122334455667788")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 0x1122334455667788 {
		t.Fatalf("parsed value mismatch: got=%x expected=1122334455667788", v)
	}

	if _, err := parseHexFingerprint("abcd"); err == nil {
		t.Fatal("expected error for short length")
	}
	if _, err := parseHexFingerprint("zzzzzzzzzzzzzzzz"); err == nil {
		t.Fatal("expected error for invalid chars")
	}
}
