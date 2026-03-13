package probabilistics

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	mrand "math/rand"
	"time"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
)

type BloomFilter struct {
	// Očekivani broj elemenata za koji je filter dimenzionisan
	n uint

	// Stopa lažno-pozitivnih (0 < p < 1)
	p float64

	// Broj bitova u bitset-u
	m uint

	// Broj hash funkcija
	k uint

	// Bitset, bit i je na poziciji (i/8) u bajtu i (i%8) unutar bajta
	bits []byte

	// Hash funkcije (seed-ovi se serijalizuju i čuvaju zajedno sa filterom)
	hashFunctions []HashWithSeed
}

// NewBloomFilter pravi bloom filter koristeći konfiguraciju i očekivani broj elemenata
// seeds može biti nil; tada se seed-ovi generišu i čuvaju u filteru
// Ako seeds postoji ali dužina nije jednaka k, generišu se novi seed-ovi
func NewBloomFilter(expectedElements uint, cfg config.BloomFilterConfig, seeds [][]byte) *BloomFilter {
	p := float64(cfg.FalsePositiveRate)
	if p <= 0 || p >= 1 {
		// Default ako konfiguracija fali ili nije ispravna
		p = 0.01
	}
	return NewBloomFilterWithParams(expectedElements, p, seeds)
}

// NewBloomFilterWithParams pravi bloom filter sa parametrima
// expectedElements mora biti > 0, a ako je 0, tretira se kao 1 da filter ostane upotrebljiv
func NewBloomFilterWithParams(expectedElements uint, falsePositiveRate float64, seeds [][]byte) *BloomFilter {
	if expectedElements == 0 {
		expectedElements = 1
	}
	p := falsePositiveRate
	if p <= 0 || p >= 1 {
		p = 0.01
	}

	// Izračunamo m i k prema standardnim formulama
	m := calculateM(expectedElements, p)
	k := calculateK(m, expectedElements)
	if k == 0 {
		k = 1
	}

	// Bitset u bajtovima
	byteLen := int((m + 7) / 8)
	bits := make([]byte, byteLen)

	// Ako nema seed-ova ili ih nema dovoljno, generišemo ih
	if seeds == nil || len(seeds) != int(k) {
		seeds = generateSeeds(int(k), 32)
	}

	return &BloomFilter{
		n:             expectedElements,
		p:             p,
		m:             m,
		k:             k,
		bits:          bits,
		hashFunctions: CreateHashFunctions(seeds),
	}
}

// Add dodaje element u filter
func (b *BloomFilter) Add(data []byte) {
	if b == nil || b.m == 0 || b.k == 0 {
		return
	}
	for i := uint(0); i < b.k; i++ {
		pos := b.hashFunctions[i].Hash(data) % uint64(b.m)
		b.setBit(uint(pos))
	}
}

// AddString dodaje string kao element u filter
func (b *BloomFilter) AddString(s string) {
	b.Add([]byte(s))
}

// MightContain proverava da li element možda postoji u filteru
// Vraća false ako sigurno ne postoji, a true ako možda postoji
func (b *BloomFilter) MightContain(data []byte) bool {
	if b == nil || b.m == 0 || b.k == 0 {
		return false
	}
	for i := uint(0); i < b.k; i++ {
		pos := b.hashFunctions[i].Hash(data) % uint64(b.m)
		if !b.getBit(uint(pos)) {
			return false
		}
	}
	return true
}

// MightContainString radi isto kao MightContain, ali za string
func (b *BloomFilter) MightContainString(s string) bool {
	return b.MightContain([]byte(s))
}

// Merge radi merge dva bloom filtera OR-ovanjem bitset-ova
// Filteri moraju bit kompatibilni, znaci bice isti n, p, m, k i isti seed-ovi
func (b *BloomFilter) Merge(other *BloomFilter) error {
	if b == nil || other == nil {
		return errors.New("nil bloom filter")
	}
	if b.m != other.m || b.k != other.k || b.n != other.n || b.p != other.p {
		return fmt.Errorf("nekompatibilni bloom filteri (parametri se razlikuju)")
	}
	if len(b.hashFunctions) != len(other.hashFunctions) {
		return fmt.Errorf("nekompatibilni bloom filteri (različit broj hash funkcija)")
	}
	for i := range b.hashFunctions {
		if !bytes.Equal(b.hashFunctions[i].Seed, other.hashFunctions[i].Seed) {
			return fmt.Errorf("nekompatibilni bloom filteri (različiti seed-ovi)")
		}
	}
	if len(b.bits) != len(other.bits) {
		return fmt.Errorf("nekompatibilni bloom filteri (različita veličina bitset-a)")
	}

	for i := range b.bits {
		b.bits[i] |= other.bits[i]
	}
	return nil
}

// Clear resetuje filter, ali ostavlja parametre i hash funkcije
func (b *BloomFilter) Clear() {
	if b == nil {
		return
	}
	for i := range b.bits {
		b.bits[i] = 0
	}
}

// Params vraća glavne parametre filtera
func (b *BloomFilter) Params() (expectedElements uint, falsePositiveRate float64, mBits uint, kHashes uint) {
	if b == nil {
		return 0, 0, 0, 0
	}
	return b.n, b.p, b.m, b.k
}

// Bits vraća kopiju bitset-a
func (b *BloomFilter) Bits() []byte {
	if b == nil {
		return nil
	}
	out := make([]byte, len(b.bits))
	copy(out, b.bits)
	return out
}

// Seeds vraća seed-ove hash funkcija
func (b *BloomFilter) Seeds() [][]byte {
	if b == nil {
		return nil
	}
	out := make([][]byte, len(b.hashFunctions))
	for i, hf := range b.hashFunctions {
		s := make([]byte, len(hf.Seed))
		copy(s, hf.Seed)
		out[i] = s
	}
	return out
}

func (b *BloomFilter) setBit(pos uint) {
	byteIndex := pos / 8
	bitIndex := pos % 8
	if int(byteIndex) >= len(b.bits) {
		return
	}
	b.bits[byteIndex] |= 1 << bitIndex
}

func (b *BloomFilter) getBit(pos uint) bool {
	byteIndex := pos / 8
	bitIndex := pos % 8
	if int(byteIndex) >= len(b.bits) {
		return false
	}
	return (b.bits[byteIndex] & (1 << bitIndex)) != 0
}

// m = -n ln(p) / (ln 2)^2
func calculateM(n uint, p float64) uint {
	ln2 := math.Ln2
	mf := -float64(n) * math.Log(p) / (ln2 * ln2)
	if mf < 1 {
		mf = 1
	}
	return uint(math.Ceil(mf))
}

// k = (m/n) ln 2
func calculateK(m uint, n uint) uint {
	if n == 0 {
		return 1
	}
	kf := (float64(m) / float64(n)) * math.Ln2
	if kf < 1 {
		kf = 1
	}
	return uint(math.Ceil(kf))
}

// Generiše seed-ove
func generateSeeds(count int, seedLen int) [][]byte {
	if count <= 0 {
		return nil
	}
	if seedLen <= 0 {
		seedLen = 32
	}

	seeds := make([][]byte, count)
	ok := true
	for i := 0; i < count; i++ {
		s := make([]byte, seedLen)
		if _, err := rand.Read(s); err != nil {
			ok = false
			break
		}
		seeds[i] = s
	}
	if ok {
		return seeds
	}

	// fallback ako crypto ne radi
	r := mrand.New(mrand.NewSource(time.Now().UnixNano()))
	for i := 0; i < count; i++ {
		s := make([]byte, seedLen)
		_, _ = r.Read(s)
		seeds[i] = s
	}
	return seeds
}

// Binarna serijalizacija (da se BloomFilter može čuvati kao običan zapis u engine-u)
//
// Format (BigEndian):
//
//	4 bajta - magic "BF01"
//	u64     - expectedElements (n)
//	f64     - falsePositiveRate (p)
//	u64     - m (broj bitova)
//	u64     - k (broj hash funkcija)
//	u64     - broj seed-ova (k)
//	  ponavlja se:
//	    u64  - dužina seed-a
//	    []byte seed
//	u64     - dužina bitset-a u bajtovima
//	[]byte  - bitset
const bloomFilterMagic = "BF01"

func (b *BloomFilter) WriteTo(writer io.Writer) (int64, error) {
	if b == nil {
		return 0, errors.New("nil bloom filter")
	}

	var written int64

	// magic
	if _, err := writer.Write([]byte(bloomFilterMagic)); err != nil {
		return written, err
	}
	written += int64(len(bloomFilterMagic))

	writeU64 := func(v uint64) error {
		if err := binary.Write(writer, binary.BigEndian, v); err != nil {
			return err
		}
		written += 8
		return nil
	}
	writeF64 := func(v float64) error {
		if err := binary.Write(writer, binary.BigEndian, v); err != nil {
			return err
		}
		written += 8
		return nil
	}

	if err := writeU64(uint64(b.n)); err != nil {
		return written, err
	}
	if err := writeF64(b.p); err != nil {
		return written, err
	}
	if err := writeU64(uint64(b.m)); err != nil {
		return written, err
	}
	if err := writeU64(uint64(b.k)); err != nil {
		return written, err
	}

	// seed-ovi
	if err := writeU64(uint64(len(b.hashFunctions))); err != nil {
		return written, err
	}
	for _, hf := range b.hashFunctions {
		if err := writeU64(uint64(len(hf.Seed))); err != nil {
			return written, err
		}
		n, err := writer.Write(hf.Seed)
		if err != nil {
			return written, err
		}
		written += int64(n)
	}

	// bitset
	if err := writeU64(uint64(len(b.bits))); err != nil {
		return written, err
	}
	n, err := writer.Write(b.bits)
	if err != nil {
		return written, err
	}
	written += int64(n)

	return written, nil
}

func (b *BloomFilter) ReadFrom(reader io.Reader) (int64, error) {
	if b == nil {
		return 0, errors.New("nil bloom filter")
	}

	var read int64

	// magic
	magic := make([]byte, len(bloomFilterMagic))
	n, err := io.ReadFull(reader, magic)
	if err != nil {
		return read, err
	}
	read += int64(n)
	if string(magic) != bloomFilterMagic {
		return read, fmt.Errorf("neispravan bloom filter magic: %q", string(magic))
	}

	readU64 := func(dst *uint64) error {
		if err := binary.Read(reader, binary.BigEndian, dst); err != nil {
			return err
		}
		read += 8
		return nil
	}
	readF64 := func(dst *float64) error {
		if err := binary.Read(reader, binary.BigEndian, dst); err != nil {
			return err
		}
		read += 8
		return nil
	}

	var n64 uint64
	if err := readU64(&n64); err != nil {
		return read, err
	}
	b.n = uint(n64)

	var p64 float64
	if err := readF64(&p64); err != nil {
		return read, err
	}
	b.p = p64

	var m64 uint64
	if err := readU64(&m64); err != nil {
		return read, err
	}
	b.m = uint(m64)

	var k64 uint64
	if err := readU64(&k64); err != nil {
		return read, err
	}
	b.k = uint(k64)

	// seed-ovi
	var seedCount uint64
	if err := readU64(&seedCount); err != nil {
		return read, err
	}
	seeds := make([][]byte, seedCount)
	for i := uint64(0); i < seedCount; i++ {
		var slen uint64
		if err := readU64(&slen); err != nil {
			return read, err
		}
		s := make([]byte, slen)
		n, err := io.ReadFull(reader, s)
		if err != nil {
			return read, err
		}
		read += int64(n)
		seeds[i] = s
	}
	b.hashFunctions = CreateHashFunctions(seeds)

	// bitset
	var bitLen uint64
	if err := readU64(&bitLen); err != nil {
		return read, err
	}
	b.bits = make([]byte, bitLen)
	n, err = io.ReadFull(reader, b.bits)
	if err != nil {
		return read, err
	}
	read += int64(n)

	return read, nil
}

// ToBytes serijalizuje BloomFilter u []byte
func (b *BloomFilter) ToBytes() ([]byte, error) {
	var buf bytes.Buffer
	_, err := b.WriteTo(&buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// FromBytes deserializuje BloomFilter iz []byte
func (b *BloomFilter) FromBytes(data []byte) error {
	_, err := b.ReadFrom(bytes.NewReader(data))
	return err
}
