package shared

import (
	"encoding/binary"
	"errors"
	"fmt"
)

type RangeDelEntry struct {
	StartKey []byte
	EndKey   []byte
	SeqId    uint64
}

// EncodedSize returns size of encoded entry
func (entry *RangeDelEntry) EncodedSize() int {
	startKeyLen := uint64(len(entry.StartKey))
	endKeyLen := uint64(len(entry.EndKey))
	buf := make([]byte, binary.MaxVarintLen64)
	startVarintLen := binary.PutUvarint(buf, startKeyLen)
	endVarintLen := binary.PutUvarint(buf, endKeyLen)
	return startVarintLen + endVarintLen + len(entry.StartKey) + len(entry.EndKey) + 8
}

// EncodeTo serializes the range del index entry into the provided buffer, returns the number of bytes written
func (entry *RangeDelEntry) EncodeTo(buf []byte) int {
	startKeyLen := uint64(len(entry.StartKey))
	endKeyLen := uint64(len(entry.EndKey))
	pos := 0

	n := binary.PutUvarint(buf[pos:], startKeyLen)
	pos += n
	n = binary.PutUvarint(buf[pos:], endKeyLen)
	pos += n

	copy(buf[pos:], entry.StartKey)
	pos += len(entry.StartKey)
	copy(buf[pos:], entry.EndKey)
	pos += len(entry.EndKey)

	binary.LittleEndian.PutUint64(buf[pos:], uint64(entry.SeqId))
	pos += 8

	return pos
}

// EncodeRangeDelEntry allocates a new buffer and encodes the entry into it
func (entry *RangeDelEntry) EncodeRangeDelEntry() []byte {
	size := entry.EncodedSize()
	buf := make([]byte, size)
	entry.EncodeTo(buf)
	return buf
}

// DecodeRangeDelEntry deserializes the range del index entry from given buffer
func DecodeRangeDelEntry(buf []byte) (*RangeDelEntry, int, error) {
	fmt.Println(buf)
	if len(buf) < 5 {
		return nil, 0, errors.New("buffer too small")
	}
	pos := 0

	startKeyLength, n := binary.Uvarint(buf[pos:])
	if n <= 0 {
		return nil, 0, errors.New("invalid index key length")
	}
	pos += n
	fmt.Println(startKeyLength)
	endKeyLength, n := binary.Uvarint(buf[pos:])
	if n <= 0 {
		return nil, 0, errors.New("invalid index key length")
	}
	pos += n
	fmt.Println(endKeyLength)

	startKey := make([]byte, startKeyLength)
	copy(startKey, buf[pos:pos+int(startKeyLength)])
	pos += int(startKeyLength)
	fmt.Println(string(startKey))
	endKey := make([]byte, endKeyLength)
	copy(endKey, buf[pos:pos+int(endKeyLength)])
	pos += int(endKeyLength)
	fmt.Println(string(endKey))

	seqId := binary.LittleEndian.Uint64(buf[pos:])
	pos += 8

	return &RangeDelEntry{
		StartKey: startKey,
		EndKey:   endKey,
		SeqId:    seqId,
	}, pos, nil
}
