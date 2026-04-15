package encoders

import (
	"encoding/binary"
	"os"
	"sort"
)

// DictEncoder implements a simple dictionary-based encoding scheme
// Instead of storing full string repeatedly, each string is assigned a unique index in a dictionary
// bitWidth determines the minimal number of bits needed to represent all indices
// bitWidth allows us to represent 5 as 5 (1 byte) instead of representing it as 5 0 0 0 0 0 0 0 (8 bytes)
type DictEncoder struct {
	bitWidth   uint16
	dictionary map[string]uint64
	keys       []string
}

// NewDictEncoder creates a new DictEncoder, optionally loading from a raw byte dictionary
func NewDictEncoder(rawDict []byte) *DictEncoder {
	de := &DictEncoder{
		dictionary: make(map[string]uint64),
		keys:       make([]string, 0),
	}
	de.LoadDict(rawDict)
	return de
}

// Keys returns all keys that are in dictionary
func (de *DictEncoder) Keys() []string {
	out := make([]string, len(de.keys))
	copy(out, de.keys)
	return out
}

// Dictionary returns loaded dictionary
func (de *DictEncoder) Dictionary() map[string]uint64 {
	out := make(map[string]uint64, len(de.keys))
	for k, v := range de.dictionary {
		out[k] = v
	}
	return out
}

// Index returns index in dictionary for given key
func (de *DictEncoder) Index(key string) (uint64, bool) {
	idx, ok := de.dictionary[key]
	return idx, ok
}

// Key returns key at given index in dictionary
func (de *DictEncoder) Key(index uint64) string {
	if index < 0 || index >= uint64(len(de.keys)) {
		return ""
	}
	return de.keys[index]
}

// AddToDict adds a new key to the dictionary, returns its index and whether it was newly added
func (de *DictEncoder) AddToDict(key []byte) (uint64, bool) {
	keyStr := string(key)
	idx, ok := de.dictionary[keyStr]
	if ok {
		return idx, false
	}
	idx = uint64(len(de.keys))
	keyCpy := make([]byte, len(key))
	copy(keyCpy, key)
	de.keys = append(de.keys, string(keyCpy))
	de.dictionary[keyStr] = idx
	de.updateBitWidth()
	return idx, true
}

// Encode converts a key into its compact binary representation based on bitWidth
func (de *DictEncoder) Encode(key []byte) []byte {
	keyStr := string(key)
	idx, ok := de.dictionary[keyStr]
	if !ok {
		panic("Not found ")
	}
	bitWidth := de.bitWidth
	outSize := (bitWidth + 7) / 8
	out := make([]byte, outSize)
	for i := 0; i < int(outSize); i++ {
		out[i] = byte(idx >> uint(8*(i%8)))
	}
	extraBits := uint8(outSize*8 - bitWidth)
	if extraBits > 0 {
		out[outSize-1] &= (1 << (8 - extraBits)) - 1
	}

	return out
}

// Decode converts a binary representation back to its original key
func (de *DictEncoder) Decode(encoded []byte) string {
	bw := de.bitWidth
	outSize := int((bw + 7) / 8)
	if len(encoded) < outSize {
		panic("encoded too short")
	}
	idx := uint64(0)
	for i := 0; i < outSize; i++ {
		idx |= uint64(encoded[i]) << (8 * i)
	}
	extraBits := uint8(outSize*8 - int(bw))
	if extraBits > 0 {
		idx &= 1<<bw - 1
	}
	if idx >= uint64(len(de.keys)) {
		panic("decoded index out of range")
	}
	return de.keys[idx]
}

// LoadDict loads dictionary from sstable footer
// dictionary in sstable footer has format bitWidth(2 bytes) + keyLen + key + idx
func (de *DictEncoder) LoadDict(dict []byte) {
	if len(dict) == 0 {
		return
	}
	pos := 0
	bitWidth := binary.LittleEndian.Uint16(dict[0:2])
	de.bitWidth = bitWidth
	pos += 2
	for pos < len(dict) {
		keyLen, n := binary.Uvarint(dict[pos:])
		if n <= 0 {
			panic("invalid varint for keylen")
		}
		pos += n
		key := string(dict[pos : pos+int(keyLen)])
		pos += int(keyLen)
		de.keys = append(de.keys, key)
		idx, n := binary.Uvarint(dict[pos:])
		if n <= 0 {
			panic("invalid varint for idx")
		}
		pos += n
		de.dictionary[key] = idx
	}
}

// SaveDict serializes the dictionary into a byte slice
func (de *DictEncoder) SaveDict() []byte {
	buf := make([]byte, 0, 2+len(de.keys)*10)
	bitWidthBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(bitWidthBytes, de.bitWidth)
	buf = append(buf, bitWidthBytes...)
	for idx, key := range de.keys {
		keyBytes := []byte(key)
		keyLen := make([]byte, binary.MaxVarintLen64)
		n := binary.PutUvarint(keyLen, uint64(len(keyBytes)))
		buf = append(buf, keyLen[:n]...)
		buf = append(buf, keyBytes...)
		idxBytes := make([]byte, binary.MaxVarintLen64)
		n = binary.PutUvarint(idxBytes, uint64(idx))
		buf = append(buf, idxBytes[:n]...)
	}
	return buf
}

// updateBitWidth updates the minimal number of bits needed to store all indices
func (de *DictEncoder) updateBitWidth() {
	if len(de.keys) == 0 {
		de.bitWidth = 0
		return
	}
	if len(de.keys) == 1 {
		de.bitWidth = 1
		return
	}
	bw := 0
	maxIdx := len(de.keys) - 1
	for max := maxIdx; max > 0; max >>= 1 {
		bw++
	}
	de.bitWidth = uint16(bw)
}

// basic getters
func (de *DictEncoder) BitWidth() uint16 {
	return de.bitWidth
}

func (de *DictEncoder) SetBitWidth(bw uint16) {
	de.bitWidth = bw
}

func (de *DictEncoder) Size() int {
	return len(de.keys)
}

// BuildDictionary builds dictionary from given slice of keys
func (de *DictEncoder) BuildDictionary(keys [][]byte) {
	keysStr := make([]string, len(keys))
	for i, k := range keys {
		keysStr[i] = string(k)
	}
	sort.Strings(keysStr)
	for _, key := range keysStr {
		de.AddToDict([]byte(key))
	}
}

// WriteToFile saves the dictionary to a file at the specified offset using SaveDict
func (de *DictEncoder) WriteToFile(f *os.File, offset uint64) error {
	data := de.SaveDict()
	if _, err := f.Seek(int64(offset), 0); err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		return err
	}
	return nil
}

// ReadDictFromFile reads a dictionary from a file at the specified offset and size
func (de *DictEncoder) ReadDictFromFile(f *os.File, offset int64, size int) error {
	data := make([]byte, size)
	if _, err := f.ReadAt(data, offset); err != nil {
		return err
	}
	de.LoadDict(data)
	return nil
}
