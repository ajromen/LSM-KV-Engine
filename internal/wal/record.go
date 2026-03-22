package wal

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
)

/*
   +---------------+-----------------+---------------+---------------+-----------------+-...-+--...--+
   |    CRC (4B)   | Timestamp (8B) | Tombstone(1B) | Key Size (8B) | Value Size (8B) | Key | Value |
   +---------------+-----------------+---------------+---------------+-----------------+-...-+--...--+
   CRC = 32bit hash computed over the payload using CRC
   Key Size = Length of the Key data
   Tombstone = If this record was deleted and has a value
   Value Size = Length of the Value data
   Key = Key data
   Value = Value data
   Timestamp = Timestamp of the operation in seconds
*/

const (
	CRC_SIZE        = 4
	TIMESTAMP_SIZE  = 8
	TOMBSTONE_SIZE  = 1
	KEY_SIZE_SIZE   = 8
	VALUE_SIZE_SIZE = 8

	CRC_START        = 0
	TIMESTAMP_START  = CRC_START + CRC_SIZE
	TOMBSTONE_START  = TIMESTAMP_START + TIMESTAMP_SIZE
	KEY_SIZE_START   = TOMBSTONE_START + TOMBSTONE_SIZE
	VALUE_SIZE_START = KEY_SIZE_START + KEY_SIZE_SIZE
	KEY_START        = VALUE_SIZE_START + VALUE_SIZE_SIZE
)

type Record struct {
	Timestamp uint64
	Tombstone bool
	Key       []byte
	Value     []byte
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
