package encoders

import (
	"encoding/binary"
	"errors"
	"os"
	"sort"
)

// candidate represents a potential dictionary entry with frequency tracking
type candidate struct {
	value    []byte
	freq     uint64
	lastSeen uint64
}

// AdaptiveEncoder implements frequency-based dictionary encoding
// Only entries that reach threshold are added to dictionary
// Candidates that are not seen within windowSize entries are automatically removed
type AdaptiveEncoder struct {
	bitWidth   uint16
	keys       [][]byte
	dictionary map[string]uint64
	idToValue  map[uint64][]byte
	nextId     uint64
	candidates map[string]*candidate
	threshold  uint64
	windowSize uint64
	entryCount uint64
}

// NewAdaptiveDictEncoderFrequency creates a new encoder with frequency threshold and optional window size
func NewAdaptiveDictEncoderFrequency(threshold uint64, windowSize uint64) *AdaptiveEncoder {
	return &AdaptiveEncoder{
		keys:       make([][]byte, 0),
		dictionary: make(map[string]uint64),
		idToValue:  make(map[uint64][]byte),
		candidates: make(map[string]*candidate),
		nextId:     0,
		threshold:  threshold,
		windowSize: windowSize,
	}
}

// Keys returns all keys as strings
func (ade *AdaptiveEncoder) Keys() []string {
	out := make([]string, len(ade.keys))
	for i, k := range ade.keys {
		out[i] = string(k)
	}
	return out
}

// Dictionary returns a copy of the dictionary
func (ade *AdaptiveEncoder) Dictionary() map[string]uint64 {
	out := make(map[string]uint64, len(ade.dictionary))
	for k, v := range ade.dictionary {
		out[k] = v
	}
	return out
}

// AddToDict adds a key, updates frequency, promotes to dictionary if threshold is reached
func (ade *AdaptiveEncoder) AddToDict(key []byte) (uint64, bool) {
	keyStr := string(key)
	ade.entryCount++

	// Remove old candidates outside window
	if ade.windowSize > 0 {
		for k, cand := range ade.candidates {
			if ade.entryCount-cand.lastSeen > ade.windowSize {
				delete(ade.candidates, k)
			}
		}
	}

	// Already in dictionary
	if idx, ok := ade.dictionary[keyStr]; ok {
		return idx, false
	}

	// Update candidate frequency
	cand, ok := ade.candidates[keyStr]
	if !ok {
		cand = &candidate{value: key, freq: 1, lastSeen: ade.entryCount}
		ade.candidates[keyStr] = cand
	} else {
		cand.freq++
		cand.lastSeen = ade.entryCount
	}

	// Promote to dictionary if threshold reached
	if cand.freq >= ade.threshold {
		idx := ade.nextId
		ade.nextId++
		ade.keys = append(ade.keys, key)
		ade.dictionary[keyStr] = idx
		ade.idToValue[idx] = key
		delete(ade.candidates, keyStr)
		ade.updateBitWidth()
		return idx, true
	}

	return 0, false
}

// Encode returns compact binary representation of a dictionary key
// If key is not in dictionary, encodes it raw with high bit 0
func (ade *AdaptiveEncoder) Encode(key []byte) []byte {
	keyStr := string(key)
	var out []byte
	outSize := (ade.bitWidth + 7) / 8

	if idx, ok := ade.dictionary[keyStr]; ok {
		// Encoded case, set high bit 1
		out = make([]byte, outSize)
		for i := 0; i < int(outSize); i++ {
			out[i] = byte(idx >> uint(8*i))
		}
		extraBits := uint8(outSize*8 - ade.bitWidth)
		if extraBits > 0 {
			out[outSize-1] &= (1 << (8 - extraBits)) - 1
		}
		out = append([]byte{1}, out...) // high bit = 1
	} else {
		// Raw value, prefix with 0
		out = append([]byte{0}, key...)
	}
	return out
}

// Decode returns original key from encoded bytes
func (ade *AdaptiveEncoder) Decode(encoded []byte) ([]byte, error) {
	if len(encoded) == 0 {
		return nil, errors.New("empty encoded data")
	}
	if encoded[0] == 0 {
		// Raw value
		return encoded[1:], nil
	}

	// Encoded dictionary value
	outSize := (ade.bitWidth + 7) / 8
	if len(encoded) < int(outSize)+1 {
		return nil, errors.New("encoded too short")
	}
	idx := uint64(0)
	for i := 0; i < int(outSize); i++ {
		idx |= uint64(encoded[i+1]) << (8 * i)
	}
	extraBits := uint8(outSize*8 - ade.bitWidth)
	if extraBits > 0 {
		idx &= 1<<ade.bitWidth - 1
	}
	if idx >= uint64(len(ade.keys)) {
		panic("decoded index out of range")
	}
	return ade.keys[idx], nil
}

// BuildDictionary builds dictionary from a slice of keys using frequency
func (ade *AdaptiveEncoder) BuildDictionary(keys [][]byte) {
	for _, k := range keys {
		ade.AddToDict(k)
	}
	// Sort keys for deterministic encoding
	sort.SliceStable(ade.keys, func(i, j int) bool {
		return string(ade.keys[i]) < string(ade.keys[j])
	})
	// Rebuild dictionary indices
	for idx, k := range ade.keys {
		ade.dictionary[string(k)] = uint64(idx)
		ade.idToValue[uint64(idx)] = k
	}
	ade.updateBitWidth()
}

// SaveDict serializes dictionary
func (ade *AdaptiveEncoder) SaveDict() []byte {
	buf := make([]byte, 0, 2+len(ade.keys)*10)
	bwBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(bwBytes, ade.bitWidth)
	buf = append(buf, bwBytes...)
	for idx, k := range ade.keys {
		keyLen := make([]byte, binary.MaxVarintLen64)
		n := binary.PutUvarint(keyLen, uint64(len(k)))
		buf = append(buf, keyLen[:n]...)
		buf = append(buf, k...)
		idxBytes := make([]byte, binary.MaxVarintLen64)
		n = binary.PutUvarint(idxBytes, uint64(idx))
		buf = append(buf, idxBytes[:n]...)
	}
	return buf
}

// ReadDictionary loads dictionary from byte slice
func (ade *AdaptiveEncoder) ReadDictionary(data []byte) error {
	ade.keys = ade.keys[:0]
	ade.dictionary = make(map[string]uint64)
	ade.idToValue = make(map[uint64][]byte)
	if len(data) == 0 {
		return nil
	}
	pos := 0
	ade.bitWidth = binary.LittleEndian.Uint16(data[pos : pos+2])
	pos += 2
	for pos < len(data) {
		keyLen, n := binary.Uvarint(data[pos:])
		if n <= 0 {
			return errors.New("invalid key length")
		}
		pos += n
		key := data[pos : pos+int(keyLen)]
		pos += int(keyLen)
		idx, n := binary.Uvarint(data[pos:])
		if n <= 0 {
			return errors.New("invalid index")
		}
		pos += n
		ade.keys = append(ade.keys, key)
		ade.dictionary[string(key)] = idx
		ade.idToValue[idx] = key
		if idx >= ade.nextId {
			ade.nextId = idx + 1
		}
	}
	return nil
}

// updateBitWidth calculates minimal bits needed
func (ade *AdaptiveEncoder) updateBitWidth() {
	if len(ade.keys) == 0 {
		ade.bitWidth = 0
		return
	}
	bw := 0
	maxIdx := len(ade.keys) - 1
	for maxi := maxIdx; maxi > 0; maxi >>= 1 {
		bw++
	}
	ade.bitWidth = uint16(bw)
}

// WriteToFile writes dictionary to file
func (ade *AdaptiveEncoder) WriteToFile(f *os.File, offset int64) error {
	data := ade.SaveDict()
	if _, err := f.Seek(offset, 0); err != nil {
		return err
	}
	_, err := f.Write(data)
	return err
}

// ReadFromFile reads dictionary from file
func (ade *AdaptiveEncoder) ReadFromFile(f *os.File, offset int64, size int) error {
	data := make([]byte, size)
	if _, err := f.ReadAt(data, offset); err != nil {
		return err
	}
	return ade.ReadDictionary(data)
}
