package wal

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

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
	CRC       uint32
	Timestamp uint64
	Tombstone bool
	KeySize   uint64
	ValueSize uint64
	Key       []byte
	Value     []byte
}

func Encode(r Record) []byte {
	r.KeySize = uint64(len(r.Key))
	r.ValueSize = uint64(len(r.Value))

	total := KEY_START + int(r.KeySize) + int(r.ValueSize)
	buf := make([]byte, total)

	binary.BigEndian.PutUint64(buf[TIMESTAMP_START:], r.Timestamp)

	if r.Tombstone {
		buf[TOMBSTONE_START] = 1
	} else {
		buf[TOMBSTONE_START] = 0
	}

	binary.BigEndian.PutUint64(buf[KEY_SIZE_START:], r.KeySize)
	binary.BigEndian.PutUint64(buf[VALUE_SIZE_START:], r.ValueSize)

	copy(buf[KEY_START:KEY_START+int(r.KeySize)], r.Key)
	copy(buf[KEY_START+int(r.KeySize):], r.Value)

	r.CRC = CRC32(buf[TIMESTAMP_START:])
	//r.CRC = crc32.ChecksumIEEE(buf[TIMESTAMP_START:])

	binary.BigEndian.PutUint32(buf[CRC_START:], r.CRC)

	return buf
}

func Decode(buf []byte) (Record, error) {
	var r Record
	r.CRC = binary.BigEndian.Uint32(buf[CRC_START:TIMESTAMP_START])

	calculatedCRC := CRC32(buf[TIMESTAMP_START:])
	if r.CRC != calculatedCRC {
		return r, fmt.Errorf("crc mismatch: received %d, expected %d", r.CRC, calculatedCRC)
	}

	r.Timestamp = binary.BigEndian.Uint64(buf[TIMESTAMP_START:TOMBSTONE_START])
	r.Tombstone = buf[TOMBSTONE_START] == 1
	r.KeySize = binary.BigEndian.Uint64(buf[KEY_SIZE_START:VALUE_SIZE_START])
	r.ValueSize = binary.BigEndian.Uint64(buf[VALUE_SIZE_START:KEY_START])

	keyEnd := KEY_START + int(r.KeySize)
	valueEnd := keyEnd + int(r.ValueSize)

	r.Key = make([]byte, r.KeySize)
	r.Value = make([]byte, r.ValueSize)

	copy(r.Key, buf[KEY_START:keyEnd])
	copy(r.Value, buf[keyEnd:valueEnd])

	return r, nil
}

func CRC32(data []byte) uint32 {
	return crc32.ChecksumIEEE(data)
}
