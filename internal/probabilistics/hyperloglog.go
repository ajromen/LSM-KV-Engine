package probabilistics

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"math/bits"
	mrand "math/rand"
	"time"
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
	m         uint32
	registers []uint8
	hashFn    HashWithSeed
}

// NewHyperLogLog pravi novu HLL instancu koristeci konfiguraciju
// Trenutno koristim default preciznost
func NewHyperLogLog() *HyperLogLog {
	return NewHyperLogLogWithParams(DefaultHLLPrecision, nil)
}

// NewHyperLogLogWithPrecision pravi HLL sa zadatom preciznoscu, seed se generise
func NewHyperLogLogWithPrecision(precision uint8) *HyperLogLog {
	return NewHyperLogLogWithParams(precision, nil)
}

// NewHyperLogLogWithParams pravi HLL sa preciznoscu i seedom
// Ako je seed prazan onda se generiše se novi

func NewHyperLogLogWithParams(precision uint8, seed []byte) *HyperLogLog {
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
func (h *HyperLogLog) Params() (precision uint8, m uint32, seed []byte, stdError float64) {
	if h == nil {
		return 0, 0, nil, 0
	}
	seedCopy := make([]byte, len(h.hashFn.Seed))
	copy(seedCopy, h.hashFn.Seed)
	return h.precision, h.m, seedCopy, h.StdError()
}

// StdError vraća standardnu grešku procjene (približno 1.04/sqrt(m))
func (h *HyperLogLog) StdError() float64 {
	if h == nil || h.m == 0 {
		return 0
	}
	return 1.04 / math.Sqrt(float64(h.m))
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
		rank = uint8(64-int(h.precision)) + 1
	} else {
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

// Binarna serijalizacija
// Format:
//
//	4 bajta  - magic string HLL1
//	u64      - precision (p)
//	u64      - m (broj registara treba da bude 2^p)
//	u64      - seedLen
//	[]byte   - seed
//	u64      - regsLen (m)
//	[]byte   - registri (uint8)
func (h *HyperLogLog) WriteTo(writer io.Writer) (int64, error) {
	if h == nil {
		return 0, errors.New("nil hyperloglog")
	}
	if h.m == 0 || len(h.registers) != int(h.m) {
		return 0, errors.New("neispravan hyperloglog")
	}

	var written int64

	// Pisemo magic string
	n, err := writer.Write([]byte(hyperLogLogMagic))
	if err != nil {
		return written, err
	}
	written += int64(n)

	writeU64 := func(v uint64) error {
		if err := binary.Write(writer, binary.BigEndian, v); err != nil {
			return err
		}
		written += 8
		return nil
	}

	if err := writeU64(uint64(h.precision)); err != nil {
		return written, err
	}
	if err := writeU64(uint64(h.m)); err != nil {
		return written, err
	}

	seed := h.hashFn.Seed
	if err := writeU64(uint64(len(seed))); err != nil {
		return written, err
	}
	n, err = writer.Write(seed)
	if err != nil {
		return written, err
	}
	written += int64(n)

	if err := writeU64(uint64(len(h.registers))); err != nil {
		return written, err
	}
	n, err = writer.Write(h.registers)
	if err != nil {
		return written, err
	}
	written += int64(n)

	return written, nil
}

func (h *HyperLogLog) ReadFrom(reader io.Reader) (int64, error) {
	if h == nil {
		return 0, errors.New("nil hyperloglog")
	}

	var read int64

	// magic
	magic := make([]byte, len(hyperLogLogMagic))
	n, err := io.ReadFull(reader, magic)
	if err != nil {
		return read, err
	}
	read += int64(n)
	if string(magic) != hyperLogLogMagic {
		return read, fmt.Errorf("neispravan hyperloglog magic: %q", string(magic))
	}

	readU64 := func(dst *uint64) error {
		if err := binary.Read(reader, binary.BigEndian, dst); err != nil {
			return err
		}
		read += 8
		return nil
	}

	var p64 uint64
	if err := readU64(&p64); err != nil {
		return read, err
	}
	precision := uint8(p64)
	if precision < 4 || precision > 18 {
		return read, fmt.Errorf("neispravna preciznost: %d", precision)
	}

	var m64 uint64
	if err := readU64(&m64); err != nil {
		return read, err
	}
	m := uint32(m64)
	if m != (uint32(1) << precision) {
		return read, fmt.Errorf("neispravan m: %d za preciznost %d", m, precision)
	}

	var seedLen uint64
	if err := readU64(&seedLen); err != nil {
		return read, err
	}
	seed := make([]byte, seedLen)
	n, err = io.ReadFull(reader, seed)
	if err != nil {
		return read, err
	}
	read += int64(n)

	var regsLen uint64
	if err := readU64(&regsLen); err != nil {
		return read, err
	}
	if regsLen != uint64(m) {
		return read, fmt.Errorf("neispravna duzina registara: %d (ocekivano %d)", regsLen, m)
	}
	regs := make([]uint8, regsLen)
	n, err = io.ReadFull(reader, regs)
	if err != nil {
		return read, err
	}
	read += int64(n)

	h.precision = precision
	h.m = m
	h.registers = regs
	h.hashFn = HashWithSeed{Seed: seed}

	return read, nil
}

func (h *HyperLogLog) ToBytes() ([]byte, error) {
	var buf bytes.Buffer
	_, err := h.WriteTo(&buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (h *HyperLogLog) FromBytes(data []byte) error {
	_, err := h.ReadFrom(bytes.NewReader(data))
	return err
}

// Merge operator podrška (2.3 zadatak): ADD element se zapisuje kao operand (HLM1)
// EncodeHLLAddOperand pravi merge operand za dodavanje elementa u HLL
// Format (BigEndian):
//
//	4 bajta - magic string HLM1
//	1 bajt  - kind = 1
//	u64     - len(element)
//	[]byte  - element
func EncodeHLLAddOperand(element []byte) []byte {
	var out []byte
	out = append(out, []byte(hyperLogLogMergeMagic)...)
	out = append(out, byte(hllMergeOpAdd))

	tmp := make([]byte, 8)
	binary.BigEndian.PutUint64(tmp, uint64(len(element)))
	out = append(out, tmp...)
	out = append(out, element...)
	return out
}

// DecodeHLLAddOperand vraća element iz merge-operanda.
func DecodeHLLAddOperand(op []byte) ([]byte, error) {
	if len(op) < 4+1+8 {
		return nil, errors.New("operand prekratak")
	}
	if string(op[:4]) != hyperLogLogMergeMagic {
		return nil, errors.New("neispravan operand magic")
	}
	if uint8(op[4]) != hllMergeOpAdd {
		return nil, errors.New("nepoznat operand kind")
	}
	l := binary.BigEndian.Uint64(op[5 : 5+8])
	if uint64(len(op)) != 4+1+8+l {
		return nil, errors.New("neispravna dužina operanda")
	}
	return op[13:], nil
}

func IsHLLStateBytes(data []byte) bool {
	return len(data) >= 4 && string(data[:4]) == hyperLogLogMagic
}

func IsHLLMergeOperand(data []byte) bool {
	return len(data) >= 5 && string(data[:4]) == hyperLogLogMergeMagic
}

func IsHLLAddOperand(data []byte) bool {
	return IsHLLMergeOperand(data) && uint8(data[4]) == hllMergeOpAdd
}

// - Ako je operand puno stanje (HLL1):
//   - ako existingState ne postoji onda vracamo operand (to je inicijalno stanje)
//   - ako postoji onda merge stanja (max registri)
//
// - Ako je operand ADD (HLM1):
//   - existingState mora postojat (HLL prvo mora bit kreiran)
//   - dekodujemo stanje dodamo element vratimo novo stanje

func ApplyHLLMerge(existingState []byte, operand []byte) ([]byte, error) {
	if len(operand) == 0 {
		return existingState, nil
	}

	// Operand je puno stanje
	if IsHLLStateBytes(operand) {
		if len(existingState) == 0 {
			// inicijalno stanje
			out := make([]byte, len(operand))
			copy(out, operand)
			return out, nil
		}
		return MergeHLLBytes(existingState, operand)
	}

	// Operand je ADD
	if IsHLLAddOperand(operand) {
		if len(existingState) == 0 {
			return nil, errors.New("HLL nije kreiran (nema početnog stanja)")
		}
		el, err := DecodeHLLAddOperand(operand)
		if err != nil {
			return nil, err
		}
		var h HyperLogLog
		if err := h.FromBytes(existingState); err != nil {
			return nil, err
		}
		h.Add(el)
		return h.ToBytes()
	}

	return nil, errors.New("nepoznat operand format")
}

// MergeHLLBytes radi merge dva serijalizovana HLL stanja
func MergeHLLBytes(a []byte, b []byte) ([]byte, error) {
	var ha HyperLogLog
	if err := ha.FromBytes(a); err != nil {
		return nil, err
	}
	var hb HyperLogLog
	if err := hb.FromBytes(b); err != nil {
		return nil, err
	}
	if err := ha.Merge(&hb); err != nil {
		return nil, err
	}
	return ha.ToBytes()
}

// Seed helper

func generateSeed(seedLen int) []byte {
	if seedLen <= 0 {
		seedLen = 32
	}
	seed := make([]byte, seedLen)
	if _, err := rand.Read(seed); err == nil {
		return seed
	}
	// fallback ako crypto/rand ne radi
	r := mrand.New(mrand.NewSource(time.Now().UnixNano()))
	_, _ = r.Read(seed)
	return seed
}
