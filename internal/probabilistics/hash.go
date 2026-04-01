package probabilistics

import (
	"crypto/md5"
	"encoding/binary"
)

type HashWithSeed struct {
	Seed []byte
}

func (h HashWithSeed) Hash(data []byte) uint64 {
	fn := md5.New()
	fn.Write(append(data, h.Seed...))
	return binary.BigEndian.Uint64(fn.Sum(nil))
}

func CreateHashFunctions(seeds [][]byte) []HashWithSeed {
	h := make([]HashWithSeed, len(seeds))
	for i, v := range seeds {
		h[i] = HashWithSeed{Seed: v}
	}
	return h
}
