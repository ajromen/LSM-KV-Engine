package probabilistic

import "github.com/ajromen/LSM-KV-Engine/internal/config"

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
