package probabilistics

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/bits"
	mrand "math/rand"
	"time"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
)

const (
	// Default preciznost
	DefaultHLLPrecision uint8 = 14

	// Binarni format za HLL
	hyperLogLogMagic = "HLL1"

	// Binarni format za merge
	hyperLogLogMergeMagic = "HLM1"

	// Merge tip
	hllMergeOpAdd uint8 = 1
)

type HyperLogLog struct {
	precision uint8
	m 		  uint32
	registers []uint8
	hashFn    HashWithSeed
}

// NewHyperLogLog pravi novu HLL instancu koristeci konfiguraciju
// Trenutno koristim default preciznost
func NewHyperLogLog(cfg config.HyperLogLogConfig) *HyperLogLog {
	_ = cfg
	return NewHyperLogLogWithParams(DefaultHLLPrecision, nil)
}

// NewHyperLogLogWithPrecision pravi HLL sa zadatom preciznoscu, seed se generise
func NewHyperLogLogWithPrecision(precision uint8) *HyperLogLog {
	return NewHyperLogLogWithParams(precision, nil)
}

// NewHyperLogLogWithParams pravi HLL sa preciznoscu i seedom
// Ako je seed prazan onda se generiše se novi

func NewHyperLogLogWithParams (precision uint8, seed []byte) *HyperLogLog {
	if precision < 4 || precision > 18 {
		precision = DefaultHLLPrecision
	}

	m := uint32(1) << precision

	if len(seed) == 0 {
		seed = generateSeed(32)
	}

	seedCopy := make([]byte, len(seed))
	copy(seedCopy, seed)

	return &HyperLogLog{
		precision: precision,
		m:         m,
		registers: make([]uint8, m),
		hashFn:    HashWithSeed{Seed: seedCopy},
	}
}

// Params vraca parametre HLLa
func (h *HyperLogLog) Params() (precision uint8, m uint32, seed []byte, std Error float64) {
	if h == nil{
		return 0,0, nil, 0
	}
	seedCopy := make([]byte, len(h.hashFn.Seed))
	copy(seedCopy, h.hashFn.Seed)
	return h.precision,h.m,seedCopy,h.StdError()
}

// StdError vraća standardnu grešku procjene (približno 1.04/sqrt(m))
func (h *HyperLogLog) StdError() float64 {
	if h == nil || h.m == 0 {
		return 0
	}
	return 1.04/math.Sqrt(float64(h.m))
}

// Registers vraća kopiju registara
func (h *HyperLogLog) Registers() []uint8 {
	if h == nil {
		return nil
	}
	
	out := make([]uint8, len(h.registers))
	copy(out, h.registers)
	return out
}

// Clear resetuje sve registre al ostavlja parametre tj. seed
func (h *HyperLogLog) Clear() {
	if h == nil {
		return
	}

	for i := range h.registers {
		h.registers[i] = 0
	}
}

// Add dodaje element u HLL
func (h *HyperLogLog) Add(data []byte) {
	if h == nil || h.m == 0 || h.precision == 0 {
		return
	}

	x := h.hashFn.Hash(data)

	// Indeks registra iz najznacajnijih p bitova
	idx := uint32(x >> (64 - h.precision))
	if idx >= h.m {
		idx = idx % h.m
	}

	// Rank = broj vodećih nula u preostalih (64-p) bitova + 1
	w := x << h.precision
	var rank uint8

	if w == 0 {
		rank = uint8(64 - int(h.precision)) + 1
	}
	else {
		rank = uint8(bits.LeadingZeros64(w) + 1)
	}

	if rank > h.registers[idx] {
		h.registers[idx] = rank
	}
}

// AddString dodaje string kao element u HLL
func (h *HyperLogLog) AddString(s string) {
	h.Add([]byte(s))
}

// Merge radi merge dva HLLa uz max po registrima
// HLLovi moraju bit kompatibilni(ista preciznost i isti seed)
func (h *HyperLogLog) Merge(other *HyperLogLog) error {
	if h == nil || other == nil {
		return errors.New("nil hyperloglog")
	}

	if h.precision != other.precision || h.m != other.m {
		return fmt.Errorf("nekompatibilni hyperloglog (različita preciznost)")
	}
	
	if !bytes.Equal(h.hashFn.Seed, other.hashFn.Seed) {
		return fmt.Errorf("nekompatibilni hyperloglog (različit seed)")
	}

	if len(h.registers) != len(other.registers) {
		return fmt.Errorf("nekompatibilni hyperloglog (različit broj registara)")
	}

	for i := range h.registers {
		if other.registers[i] > h.registers[i] {
			h.registers[i] = other.registers[i]
		}
	}
	return nil
}

// Count procjenjuje kardinalnost
func (h *HyperLogLog) Count() uint64 {
	if h == nil || h.m == 0 {
		return 0
	}

	m := float64(h.m)
	alpha := h.alpha()

	var sum float64
	var zeros uint32

	for _, r := range h.registers {
		if r == 0 {
			zeros++
		}
		sum += math.Ldexp(1.0, -int(r)) // 2^(-r)
	}

	est := alpha * m * m / sum

	// Small range correction
	if est <= 2.5*m {
		if zeros != 0 {
			est = m * math.Log(m/float64(zeros))
		}
		if est < 0 {
			return 0
		}
		return uint64(est + 0.5)
	}

	// Large range correction
	const two64 = 18446744073709551616.0 // 2^64
	if est > two64/30.0 {
		est = -two64 * math.Log(1.0-est/two64)
	}

	if est < 0 {
		return 0
	}
	return uint64(est + 0.5)
}

func (h *HyperLogLog) alpha() float64 {
	switch h.m {
	case 16:
		return 0.673
	case 32:
		return 0.697
	case 64:
		return 0.709
	default:
		return 0.7213 / (1.0 + 1.079/float64(h.m))
	}
}