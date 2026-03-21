package probabilistic

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/bits"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
)

const (
	// Binarni format:
	// 4B magic + u64(seedLen) + seed + u8(hasFingerprint) + u64(fingerprint)
	simHashMagic = "SMH1"

	// Standardni SimHash fingerprint: 64 bita
	SimHashBits = 64
)

type SimHash struct {
	hashFn         HashWithSeed
	fingerprint    uint64
	hasFingerprint bool
}

func NewSimHash(cfg config.SimHashConfig) *SimHash {
	return NewSimHashWithParams(cfg, nil)
}

func NewSimHashWithSeed(seed []byte) *SimHash {
	return NewSimHashWithParams(config.SimHashConfig{Enabled: true}, seed)
}

func NewSimHashWithParams(cfg config.SimHashConfig, seed []byte) *SimHash {
	_ = cfg

	if len(seed) == 0 {
		seed = generateSimHashSeed(32)
	}
	seedCopy := make([]byte, len(seed))
	copy(seedCopy, seed)

	return &SimHash{
		hashFn: HashWithSeed{Seed: seedCopy},
	}
}

func IsSimHashBytes(data []byte) bool {
	return len(data) >= 4 && string(data[:4]) == simHashMagic
}

func (s *SimHash) Seed() []byte {
	if s == nil {
		return nil
	}
	out := make([]byte, len(s.hashFn.Seed))
	copy(out, s.hashFn.Seed)
	return out
}

func (s *SimHash) Fingerprint() (uint64, bool) {
	if s == nil {
		return 0, false
	}
	return s.fingerprint, s.hasFingerprint
}

func (s *SimHash) FingerprintHex() (string, bool) {
	fp, ok := s.Fingerprint()
	if !ok {
		return "", false
	}
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, fp)
	return hex.EncodeToString(buf), true
}

func (s *SimHash) SetFingerprint(fp uint64) {
	if s == nil {
		return
	}
	s.fingerprint = fp
	s.hasFingerprint = true
}

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

func (s *SimHash) Clear() {
	if s == nil {
		return
	}
	s.fingerprint = 0
	s.hasFingerprint = false
}

func (s *SimHash) HashText(text string) uint64 {
	if s == nil {
		return 0
	}
	fp := s.compute(text)
	s.fingerprint = fp
	s.hasFingerprint = true
	return fp
}

func (s *SimHash) HashBytes(data []byte) uint64 {
	return s.HashText(string(data))
}

func (s *SimHash) DistanceTo(other *SimHash) (uint8, error) {
	if s == nil || other == nil {
		return 0, errors.New("nil simhash")
	}
	if !s.hasFingerprint || !other.hasFingerprint {
		return 0, errors.New("nedostaje fingerprint")
	}
	return HammingDistance64(s.fingerprint, other.fingerprint), nil
}

func (s *SimHash) DistanceToFingerprint(fp uint64) (uint8, error) {
	if s == nil {
		return 0, errors.New("nil simhash")
	}
	if !s.hasFingerprint {
		return 0, errors.New("nedostaje fingerprint")
	}
	return HammingDistance64(s.fingerprint, fp), nil
}

func (s *SimHash) DistanceToFingerprintHex(fpHex string) (uint8, error) {
	fp, err := parseHexFingerprint(fpHex)
	if err != nil {
		return 0, err
	}
	return s.DistanceToFingerprint(fp)
}

func ComputeSimHash(text string, seed []byte) uint64 {
	s := NewSimHashWithSeed(seed)
	return s.compute(text)
}

func HammingDistance64(a, b uint64) uint8 {
	return uint8(bits.OnesCount64(a ^ b))
}

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
