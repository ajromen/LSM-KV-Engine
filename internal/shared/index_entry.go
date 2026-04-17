package shared

import (
	"encoding/binary"
	"errors"
)

// IndexEntry IS A SINGLE RECORD WRITTEN INTO INDEX BLOCK -> IT MAPS A KEY TO THE OFFSET OF A DATA BLOCK IN THE SSTABLE FILE
type IndexEntry struct {
	Key        []byte // first key of data block
	BlockIndex uint32 // file offset of data block
}

// EncodedSize RETURNS THE NUMBER OF BYTES REQUIRED TO ENCODE THIS ENTRY
func (entry *IndexEntry) EncodedSize() int {
	keyLen := uint64(len(entry.Key))
	buf := make([]byte, binary.MaxVarintLen64)
	variantLen := binary.PutUvarint(buf, keyLen)
	return variantLen + len(entry.Key) + 4
}

// EncodeTo SERIALIZES THE INDEX ENTRY INTO THE PROVIDED BUFFER, RETURNS THE NUMBER OF BYTES WRITTEN
func (entry *IndexEntry) EncodeTo(buf []byte) int {
	keyLen := uint64(len(entry.Key))
	pos := 0

	// encode key-length as varint
	n := binary.PutUvarint(buf[pos:], keyLen)
	pos += n

	// encode key bytes
	copy(buf[pos:], entry.Key)
	pos += len(entry.Key)

	// encode index of block in data segment
	binary.LittleEndian.PutUint32(buf[pos:], entry.BlockIndex)
	pos += 4
	return pos
}

// EncodeIndexEntry ALLOCATES A NEW BUFFER AND ENCODES THE ENTRY INTO IT
func (entry *IndexEntry) EncodeIndexEntry() []byte {
	size := entry.EncodedSize()
	buf := make([]byte, size)
	entry.EncodeTo(buf)
	return buf
}

// DecodeIndexEntry DESERIALIZES INDEX ENTRY FROM GIVEN BUFFER -> RETURNS DECODED ENTRY, BYTES CONSUMED AND ERROR IF ANY
func DecodeIndexEntry(buf []byte) (*IndexEntry, int, error) {
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
	if pos+int(keyLength)+4 > len(buf) {
		return nil, 0, errors.New("buffer too small")
	}

	// decode key
	key := make([]byte, keyLength)
	copy(key, buf[pos:pos+int(keyLength)])
	pos += int(keyLength)

	// decode index of block in data segment
	blockIndex := binary.LittleEndian.Uint32(buf[pos:])
	pos += 4
	return &IndexEntry{
		Key:        key,
		BlockIndex: blockIndex,
	}, pos, nil
}
