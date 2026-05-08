package merge

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/ajromen/LSM-KV-Engine/internal/probabilistics"
)

const (
	ProbKeyPrefix byte = 0x01

	TypeBloom   byte = 0x01
	TypeCMS     byte = 0x02
	TypeHLL     byte = 0x03
	TypeSimHash byte = 0x04

	// First byte of a probabilistic value:
	// 0x00 = base state (full serialized structure)
	// 0xFF = merge operand (encoded operation)
	BaseStateMarker    byte = 0x00
	MergeOperandMarker byte = 0xFF

	// Operand operation codes
	OpAdd byte = 0x01
)

// ProbKey builds the internal key for a probabilistic structure.
func ProbKey(typ byte, name string) []byte {
	key := make([]byte, 2+len(name))
	key[0] = ProbKeyPrefix
	key[1] = typ
	copy(key[2:], name)
	return key
}

// IsProbKey returns true if the key belongs to a probabilistic structure.
func IsProbKey(key []byte) bool {
	return len(key) >= 2 && key[0] == ProbKeyPrefix
}

// ProbType returns the type byte of a probabilistic key.
func ProbType(key []byte) byte {
	if !IsProbKey(key) {
		return 0
	}
	return key[1]
}

// WrapBaseState prepends the base-state marker to a serialized structure.
func WrapBaseState(state []byte) []byte {
	out := make([]byte, 1+len(state))
	out[0] = BaseStateMarker
	copy(out[1:], state)
	return out
}

// WrapOperand encodes an add operation as a merge operand.
// Format: 0xFF | opCode (1B) | payloadLen (8B) | payload
func WrapOperand(opCode byte, payload []byte) []byte {
	out := make([]byte, 1+1+8+len(payload))
	out[0] = MergeOperandMarker
	out[1] = opCode
	binary.LittleEndian.PutUint64(out[2:], uint64(len(payload)))
	copy(out[10:], payload)
	return out
}

// UnwrapOperand decodes a merge operand.
func UnwrapOperand(data []byte) (opCode byte, payload []byte, err error) {
	if len(data) < 10 || data[0] != MergeOperandMarker {
		return 0, nil, fmt.Errorf("invalid merge operand")
	}
	opCode = data[1]
	l := binary.LittleEndian.Uint64(data[2:10])
	if uint64(len(data)) < 10+l {
		return 0, nil, fmt.Errorf("operand payload truncated")
	}
	return opCode, data[10 : 10+l], nil
}

// Operator applies a merge operand to an existing state.
type Operator interface {
	Apply(existingState []byte, operand []byte) ([]byte, error)
}

// GetOperator returns the correct operator for the given probabilistic key.
func GetOperator(key []byte) (Operator, error) {
	if !IsProbKey(key) {
		return nil, fmt.Errorf("not a probabilistic key")
	}
	switch key[1] {
	case TypeBloom:
		return &bloomOp{}, nil
	case TypeCMS:
		return &cmsOp{}, nil
	case TypeHLL:
		return &hllOp{}, nil
	default:
		return nil, fmt.Errorf("no merge operator for type 0x%02x", key[1])
	}
}

// ApplyAll takes records newest-first, reverses, and applies all operands to the base state.
// Returns the final merged state bytes (WITHOUT the marker byte).
func ApplyAll(key []byte, values [][]byte) ([]byte, error) {
	op, err := GetOperator(key)
	if err != nil {
		return nil, err
	}

	// reverse to oldest-first
	for i, j := 0, len(values)-1; i < j; i, j = i+1, j-1 {
		values[i], values[j] = values[j], values[i]
	}

	var state []byte
	for _, v := range values {
		if len(v) == 0 {
			continue
		}
		switch v[0] {
		case BaseStateMarker:
			state = append([]byte(nil), v[1:]...)
		case MergeOperandMarker:
			if state == nil {
				return nil, fmt.Errorf("merge operand before base state")
			}
			state, err = op.Apply(state, v)
			if err != nil {
				return nil, err
			}
		}
	}
	return state, nil
}

// ---- BloomFilter operator ----

type bloomOp struct{}

func (b *bloomOp) Apply(existing []byte, operand []byte) ([]byte, error) {
	_, payload, err := UnwrapOperand(operand)
	if err != nil {
		return nil, err
	}
	bf := &probabilistics.BloomFilter{}
	if err := bf.FromBytes(existing); err != nil {
		return nil, fmt.Errorf("decode bloom filter: %w", err)
	}
	bf.Add(payload)
	return bf.ToBytes()
}

// ---- CountMinSketch operator ----

type cmsOp struct{}

func (c *cmsOp) Apply(existing []byte, operand []byte) ([]byte, error) {
	_, payload, err := UnwrapOperand(operand)
	if err != nil {
		return nil, err
	}
	cms := &probabilistics.CountMinSketch{}
	if _, err := cms.ReadFrom(bytes.NewReader(existing)); err != nil {
		return nil, fmt.Errorf("decode cms: %w", err)
	}
	cms.Increment(payload)
	var buf bytes.Buffer
	if _, err := cms.WriteTo(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ---- HyperLogLog operator ----

type hllOp struct{}

func (h *hllOp) Apply(existing []byte, operand []byte) ([]byte, error) {
	_, payload, err := UnwrapOperand(operand)
	if err != nil {
		return nil, err
	}
	return probabilistics.ApplyHLLMerge(existing, probabilistics.EncodeHLLAddOperand(payload))
}
