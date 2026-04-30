package wal

import (
	"encoding/binary"
	"errors"
	"hash/crc32"

	"github.com/ajromen/LSM-KV-Engine/internal/enums"
)

/*

	| CRC (4B) | ExpiresAt (8B) | FragType (1B) | OpType (1B) | RecType (1B) | TxnID (8B) | SeqId (8B) | Key Size (8B) | Value Size (8B) | Key | Value |

    CRC = 32bit hash computed over the rest of the record using CRC
	ExpiresAt = Record expiry time (0 if it doesn't have it)
	FragType = Fragment type of the record (FULL, FIRST, MIDDLE, LAST)
    OpType = Type of operation: put, del, range-del...
	RecType = Record Type (SINGLE, START, TRANSACTION, COMMIT)
	TxnID = Transaction ID
	SequenceID = Sequence ID
	Key Size = Length of the Key data
   	Value Size = Length of the Value data
	Key = Key data
	Value = Value data
*/

const (
	// 47
	CRC_SIZE        = 4
	EXPIRES_AT_SIZE = 8
	FRAGTYPE_SIZE   = 1
	OPTYPE_SIZE     = 1
	RECTYPE_SIZE    = 1
	TXNID_SIZE      = 8
	SEQUENCEID_SIZE = 8
	KEY_SIZE_SIZE   = 8
	VALUE_SIZE_SIZE = 8

	CRC_START        = 0
	EXPIRES_AT_START = CRC_START + CRC_SIZE
	FRAGTYPE_START   = EXPIRES_AT_START + EXPIRES_AT_SIZE
	OPTYPE_START     = FRAGTYPE_START + FRAGTYPE_SIZE
	RECTYPE_START    = OPTYPE_START + OPTYPE_SIZE
	TXNID_START      = RECTYPE_START + RECTYPE_SIZE
	SEQUENCEID_START = TXNID_START + TXNID_SIZE
	KEY_SIZE_START   = SEQUENCEID_START + SEQUENCEID_SIZE
	VALUE_SIZE_START = KEY_SIZE_START + KEY_SIZE_SIZE
	KEY_START        = VALUE_SIZE_START + VALUE_SIZE_SIZE
)

type FragmentType uint8

const (
	FULL = iota
	FIRST
	MIDDLE
	LAST
)

type RecordType uint8

const (
	SINGLE = iota
	START
	TRANSACTION
	COMMIT
)

type Record struct {
	ExpiresAt int64
	SeqId     uint64
	OpType    enums.OpType
	Key       []byte
	Value     []byte
	//ttl 	   int64
}

type WALRecord struct {
	FragType  FragmentType
	RecType   RecordType
	TxnID     uint64
	KeySize   uint64
	ValueSize uint64
	Record    Record
}

func Encode(r WALRecord) []byte {
	totalSize := KEY_START + len(r.Record.Key) + len(r.Record.Value)
	buf := make([]byte, totalSize)

	binary.LittleEndian.PutUint64(buf[EXPIRES_AT_START:], uint64(r.Record.ExpiresAt))
	buf[FRAGTYPE_START] = byte(r.FragType)
	buf[OPTYPE_START] = byte(r.Record.OpType)
	buf[RECTYPE_START] = byte(r.RecType)
	binary.LittleEndian.PutUint64(buf[TXNID_START:SEQUENCEID_START], r.TxnID)
	binary.LittleEndian.PutUint64(buf[SEQUENCEID_START:KEY_SIZE_START], r.Record.SeqId)
	binary.LittleEndian.PutUint64(buf[KEY_SIZE_START:VALUE_SIZE_START], uint64(len(r.Record.Key)))
	binary.LittleEndian.PutUint64(buf[VALUE_SIZE_START:KEY_START], uint64(len(r.Record.Value)))
	copy(buf[KEY_START:], r.Record.Key)
	copy(buf[KEY_START+len(r.Record.Key):], r.Record.Value)

	binary.LittleEndian.PutUint32(buf[CRC_START:], CRC32(buf[EXPIRES_AT_START:]))
	return buf
}

func Decode(buf []byte) (WALRecord, error) {
	r := WALRecord{}

	crc := binary.LittleEndian.Uint32(buf[CRC_START:EXPIRES_AT_START])
	if crc != CRC32(buf[EXPIRES_AT_START:]) {
		return r, errors.New("crc mismatch")
	}

	r.Record.ExpiresAt = int64(binary.LittleEndian.Uint64(buf[EXPIRES_AT_START:FRAGTYPE_START]))

	switch buf[FRAGTYPE_START] {
	case FULL:
		r.FragType = FULL
	case FIRST:
		r.FragType = FIRST
	case MIDDLE:
		r.FragType = MIDDLE
	case LAST:
		r.FragType = LAST
	}

	r.Record.OpType = enums.OpType(buf[OPTYPE_START])

	switch buf[RECTYPE_START] {
	case SINGLE:
		r.RecType = SINGLE
	case START:
		r.RecType = START
	case TRANSACTION:
		r.RecType = TRANSACTION
	case COMMIT:
		r.RecType = COMMIT
	}

	r.TxnID = binary.LittleEndian.Uint64(buf[TXNID_START:SEQUENCEID_START])
	r.Record.SeqId = binary.LittleEndian.Uint64(buf[SEQUENCEID_START:KEY_SIZE_START])

	r.KeySize = binary.LittleEndian.Uint64(buf[KEY_SIZE_START:VALUE_SIZE_START])
	r.ValueSize = binary.LittleEndian.Uint64(buf[VALUE_SIZE_START:KEY_START])

	valueStart := KEY_START + r.KeySize
	r.Record.Key = buf[KEY_START:valueStart]
	r.Record.Value = buf[valueStart : valueStart+r.ValueSize]

	return r, nil
}

func CRC32(data []byte) uint32 {
	return crc32.ChecksumIEEE(data)
}
