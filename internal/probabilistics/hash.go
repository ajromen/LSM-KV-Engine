package probabilistic

import (
	"crypto/md5"
	"encoding/binary"
)

// type HashWithSeed struct {
// 	Seed []byte
// }

// func (h HashWithSeed) Hash(data []byte) uint64 {
// 	fn := md5.New()
// 	fn.Write(append(data, h.Seed...))
// 	return binary.BigEndian.Uint64(fn.Sum(nil))
// }

// func CreateHashFunctions(k uint) []HashWithSeed {
// 	h := make([]HashWithSeed, k)
// 	ts := uint(time.Now().Unix())
// 	for i := uint(0); i < k; i++ {
// 		seed := make([]byte, 32)
// 		binary.BigEndian.PutUint32(seed, uint32(ts+i))
// 		hfn := HashWithSeed{Seed: seed}
// 		h[i] = hfn
// 	}
// 	return h
// }

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
	for i, v := range seeds{
			h[i] = HashWithSeed{Seed: v}
	}
	return h
}