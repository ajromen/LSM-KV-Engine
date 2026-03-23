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
	CRC       uint32
	FragType  FragmentType
	RecType   RecordType
	TxnID     uint64
	KeySize   uint64
	ValueSize uint64
	Record    Record
}

func Encode(r Record) []byte {

	totalSize := KEY_START + len(r.Key) + len(r.Value)
	buf := make([]byte, totalSize)

	binary.LittleEndian.PutUint64(buf[TIMESTAMP_START:], r.Timestamp)

	if r.Tombstone {
		buf[TOMBSTONE_START] = 1
	} else {
		buf[TOMBSTONE_START] = 0
	}

	binary.LittleEndian.PutUint64(buf[KEY_SIZE_START:VALUE_SIZE_START], uint64(len(r.Key)))
	binary.LittleEndian.PutUint64(buf[VALUE_SIZE_START:KEY_START], uint64(len(r.Value)))

	copy(buf[KEY_START:], r.Key)
	copy(buf[KEY_START+len(r.Key):], r.Value)

	hashed := CRC32(buf[TIMESTAMP_START:])
	binary.LittleEndian.PutUint32(buf[CRC_START:], hashed)

	return buf
}

func Decode(buf []byte) (Record, error) {
	r := Record{}

	crc := binary.LittleEndian.Uint32(buf[CRC_START:TIMESTAMP_START])
	hashed := CRC32(buf[TIMESTAMP_START:])
	if crc != hashed {
		return r, errors.New("crc mismatch")
	}

	r.Timestamp = binary.LittleEndian.Uint64(buf[TIMESTAMP_START:TOMBSTONE_START])

	if buf[TOMBSTONE_START] == 0 {
		r.Tombstone = false
	} else {
		r.Tombstone = true
	}

	keySize := binary.LittleEndian.Uint64(buf[KEY_SIZE_START:VALUE_SIZE_START])
	valueSize := binary.LittleEndian.Uint64(buf[VALUE_SIZE_START:KEY_START])

	valueStart := KEY_START + keySize

	r.Key = buf[KEY_START:valueStart]
	r.Value = buf[valueStart : valueStart+valueSize]

	return r, nil
}

func CRC32(data []byte) uint32 {
	return crc32.ChecksumIEEE(data)
}
