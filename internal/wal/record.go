package wal

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
)

/*
	+---------------+-----------------+--------------+---------------+--------------+---------------+---------------+-----------------+-...-+--...--+
	|    CRC (4B)   | Timestamp (8B)  | FragType(1B) | Tombstone(1B) | RecType(1B)  |   TxnID (8B)  | Key Size (8B) | Value Size (8B) | Key | Value |
	+---------------+-----------------+--------------+---------------+--------------+---------------+---------------+-----------------+-...-+--...--+

	CRC = 32bit hash computed over the rest of the record using CRC
	Timestamp = Timestamp of the record
	FragType = Fragment type of the record (FULL, FIRST, MIDDLE, LAST)
	Tombstone = If this record is a delete operation
	RecType = Record Type (SINGLE, START, TRANSACTION, COMMIT)
	TxnID = Transaction ID
	Key Size = Length of the Key data
   	Value Size = Length of the Value data
	Key = Key data
	Value = Value data
*/

const (
	CRC_SIZE        = 4
	TIMESTAMP_SIZE  = 8
	FRAGTYPE_SIZE   = 1
	TOMBSTONE_SIZE  = 1
	RECTYPE_SIZE    = 1
	TXNID_SIZE      = 8
	KEY_SIZE_SIZE   = 8
	VALUE_SIZE_SIZE = 8

	CRC_START        = 0
	TIMESTAMP_START  = CRC_START + CRC_SIZE
	FRAGTYPE_START   = TIMESTAMP_START + TIMESTAMP_SIZE
	TOMBSTONE_START  = FRAGTYPE_START + FRAGTYPE_SIZE
	RECTYPE_START    = TOMBSTONE_START + TOMBSTONE_SIZE
	TXNID_START      = RECTYPE_START + RECTYPE_SIZE
	KEY_SIZE_START   = TXNID_START + TXNID_SIZE
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
	Timestamp uint64
	Tombstone bool
	Key       []byte
	Value     []byte
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

	binary.LittleEndian.PutUint64(buf[TIMESTAMP_START:], r.Record.Timestamp)

	buf[FRAGTYPE_START] = byte(r.FragType)

	if r.Record.Tombstone {
		buf[TOMBSTONE_START] = 1
	} else {
		buf[TOMBSTONE_START] = 0
	}

	buf[RECTYPE_START] = byte(r.RecType)

	binary.LittleEndian.PutUint64(buf[TXNID_START:KEY_SIZE_START], r.TxnID)

	binary.LittleEndian.PutUint64(buf[KEY_SIZE_START:VALUE_SIZE_START], uint64(len(r.Record.Key)))
	binary.LittleEndian.PutUint64(buf[VALUE_SIZE_START:KEY_START], uint64(len(r.Record.Value)))

	copy(buf[KEY_START:], r.Record.Key)
	copy(buf[KEY_START+len(r.Record.Key):], r.Record.Value)

	hashed := CRC32(buf[TIMESTAMP_START:])
	binary.LittleEndian.PutUint32(buf[CRC_START:], hashed)

	return buf
}

func Decode(buf []byte) (WALRecord, error) {
	r := WALRecord{}

	crc := binary.LittleEndian.Uint32(buf[CRC_START:TIMESTAMP_START])
	hashed := CRC32(buf[TIMESTAMP_START:])
	if crc != hashed {
		return r, errors.New("crc mismatch")
	}

	r.Record.Timestamp = binary.LittleEndian.Uint64(buf[TIMESTAMP_START:FRAGTYPE_START])

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

	if buf[TOMBSTONE_START] == 0 {
		r.Record.Tombstone = false
	} else {
		r.Record.Tombstone = true
	}

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

	r.TxnID = binary.LittleEndian.Uint64(buf[TXNID_START:KEY_SIZE_START])

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
