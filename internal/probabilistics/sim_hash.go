package probabilistic

const (
	// Binarni format:
	// 4B magic + u64(seedLen) + seed + u8(hasFingerprint) + u64(fingerprint)
	simHashMagic = "SMH1"

	// Standardni SimHash fingerprint: 64 bita
	SimHashBits = 64
)
