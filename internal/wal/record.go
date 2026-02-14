package wal

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

type RecordType uint8

const (
	RecTxnStart  RecordType = 1
	RecTxnOp     RecordType = 2
	RecTxnCommit RecordType = 3
)

func (t RecordType) Valid() bool {
	return t == RecTxnStart || t == RecTxnOp || t == RecTxnCommit
}

type OpType uint8

const (
	OpPut      OpType = 1
	OpDelete   OpType = 2
	OpRangeDel OpType = 3
)

func (o OpType) Valid() bool {
	return o == OpPut || o == OpDelete || o == OpRangeDel
}

// On-disk layout (big endian):
//
// CRC        uint32   [0:4)
// Timestamp  uint64   [4:12)
// Type       uint8    [12:13)
// TxnID      uint64   [13:21)
// OpType     uint8    [21:22)   (only meaningful when Type==RecTxnOp; otherwise 0)
// KeySize    uint64   [22:30)
// ValueSize  uint64   [30:38)
// Key        []byte   [38: ...)
// Value      []byte
//
// Semantics:
// - START:  Type=RecTxnStart, TxnID set, OpType=0, KeySize=0, ValueSize=0
// - COMMIT: Type=RecTxnCommit, TxnID set, OpType=0, KeySize=0, ValueSize=0
// - OP:     Type=RecTxnOp, TxnID set, OpType in {PUT,DELETE,RANGE_DEL}
//          PUT:      Key=key, Value=value
//          DELETE:   Key=key, Value empty
//          RANGE_DEL Key=start, Value=end

const (
	CRC_SIZE       = 4
	TIMESTAMP_SIZE = 8
	TYPE_SIZE      = 1
	TXNID_SIZE     = 8
	OPTYPE_SIZE    = 1
	KEY_SIZE_SIZE  = 8
	VAL_SIZE_SIZE  = 8

	CRC_START       = 0
	TIMESTAMP_START = CRC_START + CRC_SIZE
	TYPE_START      = TIMESTAMP_START + TIMESTAMP_SIZE
	TXNID_START     = TYPE_START + TYPE_SIZE
	OPTYPE_START    = TXNID_START + TXNID_SIZE
	KEYSZ_START     = OPTYPE_START + OPTYPE_SIZE
	VALSZ_START     = KEYSZ_START + KEY_SIZE_SIZE
	KEY_START       = VALSZ_START + VAL_SIZE_SIZE
)

type Record struct {
	CRC       uint32
	Timestamp uint64

	Type  RecordType
	TxnID uint64

	// Valid only for Type==RecTxnOp
	Op OpType

	KeySize   uint64
	ValueSize uint64
	Key       []byte
	Value     []byte
}

// Constructors
func NewTxnStart(txnID uint64, ts uint64) Record {
	return Record{
		Timestamp: ts,
		Type:      RecTxnStart,
		TxnID:     txnID,
		Op:        0,
	}
}

func NewTxnCommit(txnID uint64, ts uint64) Record {
	return Record{
		Timestamp: ts,
		Type:      RecTxnCommit,
		TxnID:     txnID,
		Op:        0,
	}
}

func NewTxnPut(txnID uint64, ts uint64, key, value []byte) Record {
	return Record{
		Timestamp: ts,
		Type:      RecTxnOp,
		TxnID:     txnID,
		Op:        OpPut,
		Key:       key,
		Value:     value,
	}
}

func NewTxnDelete(txnID uint64, ts uint64, key []byte) Record {
	return Record{
		Timestamp: ts,
		Type:      RecTxnOp,
		TxnID:     txnID,
		Op:        OpDelete,
		Key:       key,
		Value:     nil,
	}
}

// Range delete: Key=start, Value=end (engine decides inclusive/exclusive semantics)
func NewTxnRangeDelete(txnID uint64, ts uint64, start, end []byte) Record {
	return Record{
		Timestamp: ts,
		Type:      RecTxnOp,
		TxnID:     txnID,
		Op:        OpRangeDel,
		Key:       start,
		Value:     end,
	}
}

func Encode(r Record) ([]byte, error) {
	if !r.Type.Valid() {
		return nil, fmt.Errorf("wal: invalid record type %d", r.Type)
	}
	if r.TxnID == 0 {
		return nil, fmt.Errorf("wal: TxnID must be non-zero")
	}
	if r.Type == RecTxnOp {
		if !r.Op.Valid() {
			return nil, fmt.Errorf("wal: invalid op type %d", r.Op)
		}
		// validate required fields by op
		switch r.Op {
		case OpPut:
			if len(r.Key) == 0 {
				return nil, fmt.Errorf("wal: put requires non-empty key")
			}
			// value may be empty
		case OpDelete:
			if len(r.Key) == 0 {
				return nil, fmt.Errorf("wal: delete requires non-empty key")
			}
			if len(r.Value) != 0 {
				// delete doesnt have value
				return nil, fmt.Errorf("wal: delete must not carry value bytes")
			}
		case OpRangeDel:
			if len(r.Key) == 0 || len(r.Value) == 0 {
				return nil, fmt.Errorf("wal: range delete requires non-empty start and end keys")
			}
		}
	} else {
		// START/COMMIT should not carry payload
		if r.Op != 0 || len(r.Key) != 0 || len(r.Value) != 0 {
			return nil, fmt.Errorf("wal: %v record must not carry op/key/value payload", r.Type)
		}
	}

	r.KeySize = uint64(len(r.Key))
	r.ValueSize = uint64(len(r.Value))

	total := KEY_START + int(r.KeySize) + int(r.ValueSize)
	buf := make([]byte, total)

	// timestamp
	binary.BigEndian.PutUint64(buf[TIMESTAMP_START:TYPE_START], r.Timestamp)

	// type / txn / op
	buf[TYPE_START] = byte(r.Type)
	binary.BigEndian.PutUint64(buf[TXNID_START:OPTYPE_START], r.TxnID)
	buf[OPTYPE_START] = byte(r.Op)

	// sizes
	binary.BigEndian.PutUint64(buf[KEYSZ_START:VALSZ_START], r.KeySize)
	binary.BigEndian.PutUint64(buf[VALSZ_START:KEY_START], r.ValueSize)

	// payload
	copy(buf[KEY_START:KEY_START+int(r.KeySize)], r.Key)
	copy(buf[KEY_START+int(r.KeySize):], r.Value)

	// crc over everything except CRC field
	crc := CRC32(buf[TIMESTAMP_START:])
	binary.BigEndian.PutUint32(buf[CRC_START:TIMESTAMP_START], crc)

	return buf, nil
}

func Decode(buf []byte) (Record, error) {
	var r Record

	if len(buf) < KEY_START {
		return r, fmt.Errorf("wal: record too short: %d", len(buf))
	}

	r.CRC = binary.BigEndian.Uint32(buf[CRC_START:TIMESTAMP_START])
	calculated := CRC32(buf[TIMESTAMP_START:])
	if r.CRC != calculated {
		return r, fmt.Errorf("crc mismatch: received %d, expected %d", r.CRC, calculated)
	}

	r.Timestamp = binary.BigEndian.Uint64(buf[TIMESTAMP_START:TYPE_START])
	r.Type = RecordType(buf[TYPE_START])
	if !r.Type.Valid() {
		return r, fmt.Errorf("wal: invalid record type %d", r.Type)
	}

	r.TxnID = binary.BigEndian.Uint64(buf[TXNID_START:OPTYPE_START])
	if r.TxnID == 0 {
		return r, fmt.Errorf("wal: TxnID must be non-zero")
	}

	r.Op = OpType(buf[OPTYPE_START])

	r.KeySize = binary.BigEndian.Uint64(buf[KEYSZ_START:VALSZ_START])
	r.ValueSize = binary.BigEndian.Uint64(buf[VALSZ_START:KEY_START])

	keyEnd := KEY_START + int(r.KeySize)
	valEnd := keyEnd + int(r.ValueSize)

	if keyEnd < KEY_START || valEnd < keyEnd || valEnd > len(buf) {
		return r, fmt.Errorf("wal: invalid sizes: key=%d value=%d total=%d",
			r.KeySize, r.ValueSize, len(buf))
	}

	if r.KeySize > 0 {
		r.Key = make([]byte, r.KeySize)
		copy(r.Key, buf[KEY_START:keyEnd])
	}
	if r.ValueSize > 0 {
		r.Value = make([]byte, r.ValueSize)
		copy(r.Value, buf[keyEnd:valEnd])
	}

	// semantic validation
	switch r.Type {
	case RecTxnStart, RecTxnCommit:
		if r.Op != 0 || r.KeySize != 0 || r.ValueSize != 0 {
			return r, fmt.Errorf("wal: %v record must not carry op/key/value", r.Type)
		}
	case RecTxnOp:
		if !r.Op.Valid() {
			return r, fmt.Errorf("wal: invalid op type %d", r.Op)
		}
		switch r.Op {
		case OpPut:
			if r.KeySize == 0 {
				return r, fmt.Errorf("wal: put requires non-empty key")
			}
		case OpDelete:
			if r.KeySize == 0 {
				return r, fmt.Errorf("wal: delete requires non-empty key")
			}
			if r.ValueSize != 0 {
				return r, fmt.Errorf("wal: delete must not carry value")
			}
		case OpRangeDel:
			if r.KeySize == 0 || r.ValueSize == 0 {
				return r, fmt.Errorf("wal: range delete requires non-empty start and end")
			}
		}
	}

	return r, nil
}

func CRC32(data []byte) uint32 {
	return crc32.ChecksumIEEE(data)
}
