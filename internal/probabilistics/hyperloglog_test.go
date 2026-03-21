package probabilistics

import (
	"bytes"
	"fmt"
	"math"
	"testing"
)

func fixedSeed(val byte, n int) []byte {
	if n <= 0 {
		n = 32
	}
	seed := make([]byte, n)
	for i := range seed {
		seed[i] = val
	}
	return seed
}

func buildHLLWithRange(precision uint8, seed []byte, prefix string, start, count int) *HyperLogLog {
	h := NewHyperLogLogWithParams(precision, seed)
	for i := start; i < start+count; i++ {
		h.AddString(fmt.Sprintf("%s-%d", prefix, i))
	}
	return h
}

func TestHyperLogLog_NewFromConfig_Defaults(t *testing.T) {

	h := NewHyperLogLog()
	if h == nil {
		t.Fatal("ocekivan non-nil HyperLogLog")
	}

	p, m, seed, stdErr := h.Params()
	if p != DefaultHLLPrecision {
		t.Fatalf("pogresna default preciznost: got=%d expected=%d", p, DefaultHLLPrecision)
	}
	expM := uint32(1) << DefaultHLLPrecision
	if m != expM {
		t.Fatalf("pogresan broj registara: got=%d expected=%d", m, expM)
	}
	if len(seed) == 0 {
		t.Fatalf("seed ne sme biti prazan")
	}

	expectedStdErr := 1.04 / math.Sqrt(float64(m))
	if math.Abs(stdErr-expectedStdErr) > 1e-12 {
		t.Fatalf("StdError nije ispravan: got=%v expected=%v", stdErr, expectedStdErr)
	}
}

func TestHyperLogLog_PrecisionBounds(t *testing.T) {
	// van opsega -> default
	hLow := NewHyperLogLogWithParams(1, fixedSeed(0x11, 32))
	pLow, _, _, _ := hLow.Params()
	if pLow != DefaultHLLPrecision {
		t.Fatalf("precision<4 treba da padne na default: got=%d expected=%d", pLow, DefaultHLLPrecision)
	}

	hHigh := NewHyperLogLogWithParams(25, fixedSeed(0x22, 32))
	pHigh, _, _, _ := hHigh.Params()
	if pHigh != DefaultHLLPrecision {
		t.Fatalf("precision>18 treba da padne na default: got=%d expected=%d", pHigh, DefaultHLLPrecision)
	}

	hOK := NewHyperLogLogWithParams(12, fixedSeed(0x33, 32))
	pOK, mOK, _, _ := hOK.Params()
	if pOK != 12 {
		t.Fatalf("valid precision nije sacuvan: got=%d expected=12", pOK)
	}
	if mOK != (1 << 12) {
		t.Fatalf("pogresan m za precision=12: got=%d expected=%d", mOK, 1<<12)
	}
}

func TestHyperLogLog_AddCountClearAndRegisters(t *testing.T) {
	h := NewHyperLogLogWithParams(14, fixedSeed(0x44, 32))

	if got := h.Count(); got != 0 {
		t.Fatalf("prazan HLL treba da ima count=0, got=%d", got)
	}

	for i := 0; i < 5000; i++ {
		h.AddString(fmt.Sprintf("user-%d", i))
	}
	// duplikati
	for i := 0; i < 3000; i++ {
		h.AddString("same-user")
	}

	c := h.Count()
	if c < 3500 || c > 7000 {
		t.Fatalf("count van ocekivanog opsega: got=%d", c)
	}

	regs := h.Registers()
	if len(regs) != int(uint32(1)<<14) {
		t.Fatalf("pogresna duzina registara: got=%d expected=%d", len(regs), 1<<14)
	}

	h.Clear()
	if got := h.Count(); got != 0 {
		t.Fatalf("posle Clear count mora biti 0, got=%d", got)
	}
	for i, v := range h.Registers() {
		if v != 0 {
			t.Fatalf("posle Clear registar[%d] nije 0", i)
		}
	}
}

func TestHyperLogLog_MergeCompatible(t *testing.T) {
	seed := fixedSeed(0x55, 32)

	h1 := buildHLLWithRange(14, seed, "A", 0, 2000)
	h2 := buildHLLWithRange(14, seed, "B", 0, 2000)

	c1 := h1.Count()
	c2 := h2.Count()
	if c1 == 0 || c2 == 0 {
		t.Fatalf("ocekivani count>0 pre merge-a: c1=%d c2=%d", c1, c2)
	}

	if err := h1.Merge(h2); err != nil {
		t.Fatalf("merge kompatibilnih HLL neuspeo: %v", err)
	}

	merged := h1.Count()
	if merged < 3000 || merged > 5000 {
		t.Fatalf("merged count van opsega: got=%d", merged)
	}
}

func TestHyperLogLog_MergeIncompatiblePrecision(t *testing.T) {
	seed := fixedSeed(0x66, 32)

	h1 := NewHyperLogLogWithParams(14, seed)
	h2 := NewHyperLogLogWithParams(12, seed)

	if err := h1.Merge(h2); err == nil {
		t.Fatalf("ocekivana greska za nekompatibilan merge (precision)")
	}
}

func TestHyperLogLog_MergeIncompatibleSeed(t *testing.T) {
	h1 := NewHyperLogLogWithParams(14, fixedSeed(0x77, 32))
	h2 := NewHyperLogLogWithParams(14, fixedSeed(0x88, 32))

	if err := h1.Merge(h2); err == nil {
		t.Fatalf("ocekivana greska za nekompatibilan merge (seed)")
	}
}

func TestHyperLogLog_ToBytesFromBytesRoundTrip(t *testing.T) {
	h := NewHyperLogLogWithParams(14, fixedSeed(0x99, 32))
	for i := 0; i < 1500; i++ {
		h.AddString(fmt.Sprintf("k-%d", i))
	}

	b, err := h.ToBytes()
	if err != nil {
		t.Fatalf("ToBytes error: %v", err)
	}
	if !IsHLLStateBytes(b) {
		t.Fatalf("serijalizovani bajtovi treba da budu prepoznati kao HLL state")
	}

	var h2 HyperLogLog
	if err := h2.FromBytes(b); err != nil {
		t.Fatalf("FromBytes error: %v", err)
	}

	b2, err := h2.ToBytes()
	if err != nil {
		t.Fatalf("ToBytes(h2) error: %v", err)
	}

	if !bytes.Equal(b, b2) {
		t.Fatalf("binary round-trip promenio je sadrzaj")
	}

	if h.Count() != h2.Count() {
		t.Fatalf("count nije isti posle round-trip: c1=%d c2=%d", h.Count(), h2.Count())
	}
}

func TestHyperLogLog_FromBytesInvalidMagic(t *testing.T) {
	var h HyperLogLog
	if err := h.FromBytes([]byte("BAD!")); err == nil {
		t.Fatalf("ocekivana greska za neispravan magic")
	}
}

func TestHyperLogLog_EncodeDecodeAddOperand(t *testing.T) {
	op := EncodeHLLAddOperand([]byte("abc"))
	if !IsHLLMergeOperand(op) {
		t.Fatalf("operand mora biti HLL merge operand")
	}
	if !IsHLLAddOperand(op) {
		t.Fatalf("operand mora biti HLL add operand")
	}

	el, err := DecodeHLLAddOperand(op)
	if err != nil {
		t.Fatalf("DecodeHLLAddOperand error: %v", err)
	}
	if string(el) != "abc" {
		t.Fatalf("decoded element nije isti: got=%q expected=%q", string(el), "abc")
	}
}

func TestHyperLogLog_DecodeAddOperandInvalid(t *testing.T) {
	// prekratko
	if _, err := DecodeHLLAddOperand([]byte{1, 2, 3}); err == nil {
		t.Fatalf("ocekivana greska za prekratak operand")
	}

	// loš magic
	badMagic := append([]byte("BAD!"), byte(hllMergeOpAdd))
	tmp := make([]byte, 8)
	binaryLen := uint64(3)
	putU64BE(tmp, binaryLen)
	badMagic = append(badMagic, tmp...)
	badMagic = append(badMagic, []byte("abc")...)
	if _, err := DecodeHLLAddOperand(badMagic); err == nil {
		t.Fatalf("ocekivana greška za los magic")
	}

	// loš kind
	badKind := append([]byte(hyperLogLogMergeMagic), byte(99))
	tmp2 := make([]byte, 8)
	putU64BE(tmp2, uint64(3))
	badKind = append(badKind, tmp2...)
	badKind = append(badKind, []byte("abc")...)
	if _, err := DecodeHLLAddOperand(badKind); err == nil {
		t.Fatalf("ocekivana greška za los kind")
	}

	// neispravna dužina
	valid := EncodeHLLAddOperand([]byte("abcd"))
	truncated := valid[:len(valid)-1]
	if _, err := DecodeHLLAddOperand(truncated); err == nil {
		t.Fatalf("ocekivana greška za neispravnu duzinu operanda")
	}
}

func TestHyperLogLog_ApplyHLLMerge_WithAddOperand(t *testing.T) {
	h := NewHyperLogLogWithParams(14, fixedSeed(0xCD, 32))
	state, err := h.ToBytes()
	if err != nil {
		t.Fatalf("ToBytes error: %v", err)
	}

	op := EncodeHLLAddOperand([]byte("one"))
	mergedState, err := ApplyHLLMerge(state, op)
	if err != nil {
		t.Fatalf("ApplyHLLMerge (ADD) error: %v", err)
	}

	var out HyperLogLog
	if err := out.FromBytes(mergedState); err != nil {
		t.Fatalf("FromBytes merged state error: %v", err)
	}
	if out.Count() == 0 {
		t.Fatalf("count posle ADD merge operanda mora biti > 0")
	}

	// ADD bez postojeceg stanja mora da padne
	if _, err := ApplyHLLMerge(nil, op); err == nil {
		t.Fatalf("ocekivana greška za ADD operand bez postojeceg stanja")
	}
}

func TestHyperLogLog_ApplyHLLMerge_StateOperandAndUnknown(t *testing.T) {
	seed := fixedSeed(0xEF, 32)
	h1 := buildHLLWithRange(14, seed, "S1", 0, 1500)
	h2 := buildHLLWithRange(14, seed, "S2", 0, 1500)

	s1, err := h1.ToBytes()
	if err != nil {
		t.Fatalf("ToBytes s1 error: %v", err)
	}
	s2, err := h2.ToBytes()
	if err != nil {
		t.Fatalf("ToBytes s2 error: %v", err)
	}

	// existing + state
	merged, err := ApplyHLLMerge(s1, s2)
	if err != nil {
		t.Fatalf("ApplyHLLMerge state+state error: %v", err)
	}
	var hm HyperLogLog
	if err := hm.FromBytes(merged); err != nil {
		t.Fatalf("FromBytes merged error: %v", err)
	}
	c := hm.Count()
	if c < 2200 || c > 3800 {
		t.Fatalf("count posle state+state merge van opsega: got=%d", c)
	}

	// nil existing + state => copy state
	cloned, err := ApplyHLLMerge(nil, s1)
	if err != nil {
		t.Fatalf("ApplyHLLMerge nil+state error: %v", err)
	}
	if !bytes.Equal(cloned, s1) {
		t.Fatalf("nil+state treba da vrati isti sadrzaj stanja")
	}

	// unknown operand
	if _, err := ApplyHLLMerge(s1, []byte("????")); err == nil {
		t.Fatalf("ocekivana greska za unknown operand format")
	}

	// empty operand => vraća existing bez promene
	same, err := ApplyHLLMerge(s1, nil)
	if err != nil {
		t.Fatalf("ApplyHLLMerge empty operand error: %v", err)
	}
	if !bytes.Equal(same, s1) {
		t.Fatalf("empty operand treba da vrati existing state bez promjene")
	}
}

func TestHyperLogLog_MergeHLLBytes(t *testing.T) {
	seed := fixedSeed(0xA1, 32)
	a := buildHLLWithRange(14, seed, "A", 0, 1000)
	b := buildHLLWithRange(14, seed, "B", 0, 1000)

	ba, err := a.ToBytes()
	if err != nil {
		t.Fatalf("ToBytes(a) error: %v", err)
	}
	bb, err := b.ToBytes()
	if err != nil {
		t.Fatalf("ToBytes(b) error: %v", err)
	}

	merged, err := MergeHLLBytes(ba, bb)
	if err != nil {
		t.Fatalf("MergeHLLBytes error: %v", err)
	}

	var out HyperLogLog
	if err := out.FromBytes(merged); err != nil {
		t.Fatalf("FromBytes merged error: %v", err)
	}
	c := out.Count()
	if c < 1400 || c > 2600 {
		t.Fatalf("count posle MergeHLLBytes van opsega: got=%d", c)
	}
}

func TestHyperLogLog_MergeHLLBytesIncompatibleSeed(t *testing.T) {
	a := buildHLLWithRange(14, fixedSeed(0x01, 32), "A", 0, 100)
	b := buildHLLWithRange(14, fixedSeed(0x02, 32), "B", 0, 100)

	ba, _ := a.ToBytes()
	bb, _ := b.ToBytes()

	if _, err := MergeHLLBytes(ba, bb); err == nil {
		t.Fatalf("ocekivana greška za MergeHLLBytes sa različitim seed-ovima")
	}
}

// helper da izbjegnemo dodatni import samo zbog jedne put funkcije
func putU64BE(dst []byte, v uint64) {
	_ = dst[7]
	dst[0] = byte(v >> 56)
	dst[1] = byte(v >> 48)
	dst[2] = byte(v >> 40)
	dst[3] = byte(v >> 32)
	dst[4] = byte(v >> 24)
	dst[5] = byte(v >> 16)
	dst[6] = byte(v >> 8)
	dst[7] = byte(v)
}
