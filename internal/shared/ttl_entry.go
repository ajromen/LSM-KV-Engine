package shared

import (
	"encoding/binary"
	"errors"
)

type TTLEntry struct {
	Key       []byte
	ExpiresAt int64
}

func (entry *TTLEntry) EncodedSize() int {
	keyLen := uint64(len(entry.Key))
	buf := make([]byte, binary.MaxVarintLen64)
	variantLen := binary.PutUvarint(buf, keyLen)
	return variantLen + len(entry.Key) + 8
}

// EncodeTo SERIALIZES THE TTL INDEX ENTRY INTO THE PROVIDED BUFFER, RETURNS THE NUMBER OF BYTES WRITTEN
func (entry *TTLEntry) EncodeTo(buf []byte) int {
	keyLen := uint64(len(entry.Key))
	pos := 0

	// encode key-length as varint
	n := binary.PutUvarint(buf[pos:], keyLen)
	pos += n

	// encode key bytes
	copy(buf[pos:], entry.Key)
	pos += len(entry.Key)

	// encode ttl
	binary.LittleEndian.PutUint64(buf[pos:], uint64(entry.ExpiresAt))
	pos += 8

	return pos
}

// EncodeTTLIndexEntry ALLOCATES A NEW BUFFER AND ENCODES THE ENTRY INTO IT
func (entry *TTLEntry) EncodeTTLIndexEntry() []byte {
	size := entry.EncodedSize()
	buf := make([]byte, size)
	entry.EncodeTo(buf)
	return buf
}

// DecodeTTLIndexEntry DESERIALIZES TTL INDEX ENTRY FROM GIVEN BUFFER -> RETURNS DECODED ENTRY, BYTES CONSUMED AND ERROR IF ANY
func DecodeTTLIndexEntry(buf []byte) (*TTLEntry, int, error) {
	if len(buf) < 5 {
		return nil, 0, errors.New("buffer too small")
	}
	pos := 0

	// decode key length
	keyLength, n := binary.Uvarint(buf[pos:])
	if n <= 0 {
		return nil, 0, errors.New("invalid index key length")
	}
	pos += n
	if pos+int(keyLength)+8 > len(buf) {
		return nil, 0, errors.New("buffer too small")
	}

	// decode key
	key := make([]byte, keyLength)
	copy(key, buf[pos:pos+int(keyLength)])
	pos += int(keyLength)

	// decode index of block in data segment
	expiresAt := binary.LittleEndian.Uint64(buf[pos:])
	pos += 8
	return &TTLEntry{
		Key:       key,
		ExpiresAt: int64(expiresAt),
	}, pos, nil
}
