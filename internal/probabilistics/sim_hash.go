package probabilistic

import (
	"encoding/binary"
	"encoding/hex"
	"errors"

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
