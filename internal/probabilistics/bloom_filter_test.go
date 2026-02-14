package probabilistics

import (
	"bytes"
	"encoding/json"
	"math"
	"testing"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
)

func makeDeterministicSeeds(k int, seedLen int) [][]byte {
	seeds := make([][]byte, k)
	for i := 0; i < k; i++ {
		s := make([]byte, seedLen)
		for j := 0; j < seedLen; j++ {
			s[j] = byte(i + j)
		}
		seeds[i] = s
	}
	return seeds
}

func TestBloomFilter_CalculateMAndK(t *testing.T) {
	n := uint(1000)
	p := 0.01

	mGot := calculateM(n, p)
	kGot := calculateK(mGot, n)

	// m = -n ln(p) / (ln 2)^2
	mExpected := uint(math.Ceil(-float64(n) * math.Log(p) / (math.Ln2 * math.Ln2)))
	if mGot != mExpected {
		t.Fatalf("m nije ispravno izracunato: got=%d expected=%d", mGot, mExpected)
	}

	// k = (m/n) ln 2
	kExpected := uint(math.Ceil((float64(mExpected) / float64(n)) * math.Ln2))
	if kExpected == 0 {
		kExpected = 1
	}
	if kGot != kExpected {
		t.Fatalf("k nije ispravno izracunato: got=%d expected=%d", kGot, kExpected)
	}
}

func TestBloomFilter_AddMightContainAndClear(t *testing.T) {
	cfg := config.BloomFilterConfig{FalsePositiveRate: 0.01}
	expectedN := uint(1000)
	p := float64(cfg.FalsePositiveRate)

	m := calculateM(expectedN, p)
	k := calculateK(m, expectedN)
	seeds := makeDeterministicSeeds(int(k), 32)

	bf := NewBloomFilter(expectedN, cfg, seeds)
	if bf == nil {
		t.Fatal("bf je nil")
	}

	key := []byte("ključ-1")

	// Prazan filter mora sigurno da vrati false.
	if bf.MightContain(key) {
		t.Fatalf("prazan bloom filter ne sme da vrati true")
	}

	bf.Add(key)
	if !bf.MightContain(key) {
		t.Fatalf("dodati element mora da bude prepoznat kao prisutan")
	}

	bf.Clear()
	if bf.MightContain(key) {
		t.Fatalf("posle Clear(), element ne sme da bude prisutan")
	}
}

func TestBloomFilter_Merge(t *testing.T) {
	expectedN := uint(1000)
	p := 0.01

	m := calculateM(expectedN, p)
	k := calculateK(m, expectedN)
	seeds := makeDeterministicSeeds(int(k), 32)

	base := NewBloomFilterWithParams(expectedN, p, seeds)
	a := NewBloomFilterWithParams(expectedN, p, seeds)
	b := NewBloomFilterWithParams(expectedN, p, seeds)

	keyA := []byte("a")
	keyB := []byte("b")
	a.Add(keyA)
	b.Add(keyB)

	// Ocekivani OR pre merge-a
	expectedBits := make([]byte, len(base.bits))
	copy(expectedBits, base.bits)
	for i := range expectedBits {
		expectedBits[i] |= a.bits[i]
		expectedBits[i] |= b.bits[i]
	}

	if err := base.Merge(a); err != nil {
		t.Fatalf("merge a neuspeo: %v", err)
	}
	if err := base.Merge(b); err != nil {
		t.Fatalf("merge b neuspeo: %v", err)
	}

	if !base.MightContain(keyA) {
		t.Fatalf("posle merge, ključ A mora biti prisutan")
	}
	if !base.MightContain(keyB) {
		t.Fatalf("posle merge, ključ B mora biti prisutan")
	}

	if !bytes.Equal(base.bits, expectedBits) {
		t.Fatalf("bitset posle merge nije jednak ocekivanom OR-u")
	}
}

func TestBloomFilter_MergeIncompatible(t *testing.T) {
	expectedN := uint(1000)
	p := 0.01

	m := calculateM(expectedN, p)
	k := calculateK(m, expectedN)
	seeds1 := makeDeterministicSeeds(int(k), 32)
	seeds2 := makeDeterministicSeeds(int(k), 32)

	// Namerno promenimo jedan bajt seed-a da filteri budu nekompatibilni
	seeds2[0][0] ^= 0xFF

	bf1 := NewBloomFilterWithParams(expectedN, p, seeds1)
	bf2 := NewBloomFilterWithParams(expectedN, p, seeds2)

	if err := bf1.Merge(bf2); err == nil {
		t.Fatalf("ocekivao sam gresku za merge nekompatibilnih filtera (razliciti seed-ovi)")
	}
}

func TestBloomFilter_BinaryRoundTrip(t *testing.T) {
	expectedN := uint(500)
	p := 0.01
	m := calculateM(expectedN, p)
	k := calculateK(m, expectedN)
	seeds := makeDeterministicSeeds(int(k), 32)

	bf := NewBloomFilterWithParams(expectedN, p, seeds)
	bf.Add([]byte("x"))
	bf.Add([]byte("y"))

	data, err := bf.ToBytes()
	if err != nil {
		t.Fatalf("ToBytes error: %v", err)
	}

	var bf2 BloomFilter
	if err := bf2.FromBytes(data); err != nil {
		t.Fatalf("FromBytes error: %v", err)
	}

	if bf2.n != bf.n || bf2.p != bf.p || bf2.m != bf.m || bf2.k != bf.k {
		t.Fatalf("parametri nisu jednaki posle round-trip: got(n=%d,p=%v,m=%d,k=%d) expected(n=%d,p=%v,m=%d,k=%d)",
			bf2.n, bf2.p, bf2.m, bf2.k,
			bf.n, bf.p, bf.m, bf.k,
		)
	}

	if !bytes.Equal(bf2.bits, bf.bits) {
		t.Fatalf("bitset nije isti posle round-trip")
	}

	seeds2 := bf2.Seeds()
	if len(seeds2) != len(seeds) {
		t.Fatalf("seed count nije isti: got=%d expected=%d", len(seeds2), len(seeds))
	}
	for i := range seeds {
		if !bytes.Equal(seeds2[i], seeds[i]) {
			t.Fatalf("seed[%d] nije isti posle round-trip", i)
		}
	}

	if !bf2.MightContain([]byte("x")) || !bf2.MightContain([]byte("y")) {
		t.Fatalf("posle round-trip, dodati elementi moraju biti prepoznati")
	}
}

func TestBloomFilter_BinaryInvalidMagic(t *testing.T) {
	var bf BloomFilter
	err := bf.FromBytes([]byte("NOT!"))
	if err == nil {
		t.Fatalf("ocekivao sam gresku za neispravan magic")
	}
}

func TestBloomFilter_JSONRoundTrip(t *testing.T) {
	expectedN := uint(200)
	p := 0.01
	m := calculateM(expectedN, p)
	k := calculateK(m, expectedN)
	seeds := makeDeterministicSeeds(int(k), 32)

	bf := NewBloomFilterWithParams(expectedN, p, seeds)
	bf.Add([]byte("abc"))

	data, err := json.Marshal(bf)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var bf2 BloomFilter
	if err := json.Unmarshal(data, &bf2); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if bf2.n != bf.n || bf2.p != bf.p || bf2.m != bf.m || bf2.k != bf.k {
		t.Fatalf("parametri nisu jednaki posle JSON round-trip")
	}
	if !bytes.Equal(bf2.bits, bf.bits) {
		t.Fatalf("bitset nije isti posle JSON round-trip")
	}
	if !bf2.MightContain([]byte("abc")) {
		t.Fatalf("posle JSON round-trip, dodati element mora biti prepoznat")
	}
}
