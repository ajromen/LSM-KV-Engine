package probabilistics

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/bits"
	mrand "math/rand"
	"strings"
	"time"
	"unicode"
)

const (
	// Binarni format:
	// 4B magic + u64(seedLen) + seed + u8(hasFingerprint) + u64(fingerprint)
	simHashMagic = "SMH1"

	// Standardni SimHash fingerprint: 64 bita
	SimHashBits = 64
)

// SimHash čuva seed hash funkcije i poslednji izračunat fingerprint.
type SimHash struct {
	hashFn         HashWithSeed
	fingerprint    uint64
	hasFingerprint bool
}

// NewSimHash pravi novu instancu na osnovu konfiguracije.
func NewSimHash() *SimHash {
	return NewSimHashWithParams(nil)
}

// NewSimHashWithSeed pomoćni konstruktor kada želiš eksplicitno da zadaš seed.
func NewSimHashWithSeed(seed []byte) *SimHash {
	return NewSimHashWithParams(seed)
}

// NewSimHashWithParams pomoćni konstruktor.
func NewSimHashWithParams(seed []byte) *SimHash {

	if len(seed) == 0 {
		seed = generateSimHashSeed(32)
	}
	seedCopy := make([]byte, len(seed))
	copy(seedCopy, seed)

	return &SimHash{
		hashFn: HashWithSeed{Seed: seedCopy},
	}
}

// IsSimHashBytes proverava da li niz bajtova izgleda kao SimHash stanje.
func IsSimHashBytes(data []byte) bool {
	return len(data) >= 4 && string(data[:4]) == simHashMagic
}

// Seed vraća kopiju seed-a.
func (s *SimHash) Seed() []byte {
	if s == nil {
		return nil
	}
	out := make([]byte, len(s.hashFn.Seed))
	copy(out, s.hashFn.Seed)
	return out
}

// Fingerprint vraća trenutni fingerprint i da li postoji.
func (s *SimHash) Fingerprint() (uint64, bool) {
	if s == nil {
		return 0, false
	}
	return s.fingerprint, s.hasFingerprint
}

// FingerprintHex vraća fingerprint kao 16-hex string.
func (s *SimHash) FingerprintHex() (string, bool) {
	fp, ok := s.Fingerprint()
	if !ok {
		return "", false
	}
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, fp)
	return hex.EncodeToString(buf), true
}

// SetFingerprint direktno postavlja fingerprint.
func (s *SimHash) SetFingerprint(fp uint64) {
	if s == nil {
		return
	}
	s.fingerprint = fp
	s.hasFingerprint = true
}

// SetFingerprintHex postavlja fingerprint iz 16-hex stringa.
func (s *SimHash) SetFingerprintHex(fpHex string) error {
	if s == nil {
		return errors.New("nil simhash")
	}
	fp, err := parseHexFingerprint(fpHex)
	if err != nil {
		return err
	}
	s.fingerprint = fp
	s.hasFingerprint = true
	return nil
}

// Clear briše sačuvani fingerprint, ali ostavlja seed.
func (s *SimHash) Clear() {
	if s == nil {
		return
	}
	s.fingerprint = 0
	s.hasFingerprint = false
}

// HashText izračuna fingerprint za prosleđeni tekst i sačuva ga u instanci.
func (s *SimHash) HashText(text string) uint64 {
	if s == nil {
		return 0
	}
	fp := s.compute(text)
	s.fingerprint = fp
	s.hasFingerprint = true
	return fp
}

// HashBytes isto kao HashText za []byte.
func (s *SimHash) HashBytes(data []byte) uint64 {
	return s.HashText(string(data))
}

// DistanceTo računa Hemingovu udaljenost između dve SimHash instance.
func (s *SimHash) DistanceTo(other *SimHash) (uint8, error) {
	if s == nil || other == nil {
		return 0, errors.New("nil simhash")
	}
	if !s.hasFingerprint || !other.hasFingerprint {
		return 0, errors.New("nedostaje fingerprint")
	}
	return HammingDistance64(s.fingerprint, other.fingerprint), nil
}

// DistanceToFingerprint računa udaljenost trenutnog fingerprint-a i prosleđenog.
func (s *SimHash) DistanceToFingerprint(fp uint64) (uint8, error) {
	if s == nil {
		return 0, errors.New("nil simhash")
	}
	if !s.hasFingerprint {
		return 0, errors.New("nedostaje fingerprint")
	}
	return HammingDistance64(s.fingerprint, fp), nil
}

// DistanceToFingerprintHex računa udaljenost do fingerprint-a datog kao hex string.
func (s *SimHash) DistanceToFingerprintHex(fpHex string) (uint8, error) {
	fp, err := parseHexFingerprint(fpHex)
	if err != nil {
		return 0, err
	}
	return s.DistanceToFingerprint(fp)
}

// ComputeSimHash je stateless helper.
func ComputeSimHash(text string, seed []byte) uint64 {
	s := NewSimHashWithSeed(seed)
	return s.compute(text)
}

// HammingDistance64 vraća broj bitova koji se razlikuju između dva 64-bitna fingerprint-a.
func HammingDistance64(a, b uint64) uint8 {
	return uint8(bits.OnesCount64(a ^ b))
}

// HammingDistanceHex prima 2 heks stringa (16 hex karaktera) i računa udaljenost.
func HammingDistanceHex(aHex, bHex string) (uint8, error) {
	a, err := parseHexFingerprint(aHex)
	if err != nil {
		return 0, err
	}
	b, err := parseHexFingerprint(bHex)
	if err != nil {
		return 0, err
	}
	return HammingDistance64(a, b), nil
}

// WriteTo serijalizuje SimHash u binarni format.
func (s *SimHash) WriteTo(w io.Writer) (int64, error) {
	if s == nil {
		return 0, errors.New("nil simhash")
	}

	var written int64

	n, err := w.Write([]byte(simHashMagic))
	if err != nil {
		return written, err
	}
	written += int64(n)

	writeU64 := func(v uint64) error {
		if err := binary.Write(w, binary.BigEndian, v); err != nil {
			return err
		}
		written += 8
		return nil
	}

	seed := s.hashFn.Seed
	if err := writeU64(uint64(len(seed))); err != nil {
		return written, err
	}
	n, err = w.Write(seed)
	if err != nil {
		return written, err
	}
	written += int64(n)

	has := uint8(0)
	if s.hasFingerprint {
		has = 1
	}
	if err := binary.Write(w, binary.BigEndian, has); err != nil {
		return written, err
	}
	written += 1

	if err := writeU64(s.fingerprint); err != nil {
		return written, err
	}

	return written, nil
}

// ReadFrom deserijalizuje SimHash iz binarnog formata.
func (s *SimHash) ReadFrom(r io.Reader) (int64, error) {
	if s == nil {
		return 0, errors.New("nil simhash")
	}

	var read int64

	magic := make([]byte, len(simHashMagic))
	n, err := io.ReadFull(r, magic)
	if err != nil {
		return read, err
	}
	read += int64(n)

	if string(magic) != simHashMagic {
		return read, fmt.Errorf("neispravan simhash magic: %q", string(magic))
	}

	readU64 := func(dst *uint64) error {
		if err := binary.Read(r, binary.BigEndian, dst); err != nil {
			return err
		}
		read += 8
		return nil
	}

	var seedLen uint64
	if err := readU64(&seedLen); err != nil {
		return read, err
	}

	seed := make([]byte, seedLen)
	n, err = io.ReadFull(r, seed)
	if err != nil {
		return read, err
	}
	read += int64(n)

	var has uint8
	if err := binary.Read(r, binary.BigEndian, &has); err != nil {
		return read, err
	}
	read += 1

	var fp uint64
	if err := readU64(&fp); err != nil {
		return read, err
	}

	s.hashFn = HashWithSeed{Seed: seed}
	s.fingerprint = fp
	s.hasFingerprint = (has != 0)

	return read, nil
}

// ToBytes helper za serijalizaciju.
func (s *SimHash) ToBytes() ([]byte, error) {
	var buf bytes.Buffer
	_, err := s.WriteTo(&buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// FromBytes helper za deserijalizaciju.
func (s *SimHash) FromBytes(data []byte) error {
	_, err := s.ReadFrom(bytes.NewReader(data))
	return err
}

// compute implementira 64-bitni SimHash.
// Tokeni su lower-case reči/cifre; težina je frekvencija tokena u tekstu.
func (s *SimHash) compute(text string) uint64 {
	tokenFreq := tokenizeAndCount(text)
	if len(tokenFreq) == 0 {
		return 0
	}

	var vec [SimHashBits]int64

	for tok, weight := range tokenFreq {
		h := s.hashFn.Hash([]byte(tok))
		w := int64(weight)

		for i := 0; i < SimHashBits; i++ {
			mask := uint64(1) << uint(i)
			if (h & mask) != 0 {
				vec[i] += w
			} else {
				vec[i] -= w
			}
		}
	}

	var fp uint64
	for i := 0; i < SimHashBits; i++ {
		if vec[i] >= 0 {
			fp |= (uint64(1) << uint(i))
		}
	}
	return fp
}

// tokenizeAndCount deli tekst na tokene (slova/cifre), normalizuje na lower-case i broji frekvencije.
func tokenizeAndCount(text string) map[string]uint32 {
	freq := make(map[string]uint32)
	if len(text) == 0 {
		return freq
	}

	var b strings.Builder

	flush := func() {
		if b.Len() == 0 {
			return
		}
		tok := b.String()
		if tok != "" {
			freq[tok]++
		}
		b.Reset()
	}

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		} else {
			flush()
		}
	}
	flush()

	return freq
}

func parseHexFingerprint(h string) (uint64, error) {
	h = strings.TrimSpace(h)
	h = strings.TrimPrefix(h, "0x")
	h = strings.TrimPrefix(h, "0X")

	if len(h) != 16 {
		return 0, fmt.Errorf("hex fingerprint mora imati 16 karaktera (64 bita), dobio %d", len(h))
	}

	raw, err := hex.DecodeString(h)
	if err != nil {
		return 0, fmt.Errorf("neispravan hex fingerprint: %w", err)
	}
	return binary.BigEndian.Uint64(raw), nil
}

func generateSimHashSeed(seedLen int) []byte {
	if seedLen <= 0 {
		seedLen = 32
	}
	seed := make([]byte, seedLen)

	if _, err := rand.Read(seed); err == nil {
		return seed
	}

	r := mrand.New(mrand.NewSource(time.Now().UnixNano()))
	_, _ = r.Read(seed)
	return seed
}
