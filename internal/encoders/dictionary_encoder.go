package encoders

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type DictionaryEncoder struct {
	keys     [][]byte
	MapOfIdx map[string]int
}

func NewDictionaryEncoder() *DictionaryEncoder {
	return &DictionaryEncoder{
		keys:     make([][]byte, 0),
		MapOfIdx: make(map[string]int),
	}
}

func (encoder *DictionaryEncoder) GetKeys() [][]byte {
	out := make([][]byte, len(encoder.keys))
	copy(out, encoder.keys)
	return out
}

func (encoder *DictionaryEncoder) GetMapOfIdx() map[string]int {
	out := make(map[string]int, len(encoder.MapOfIdx))
	for k, v := range encoder.MapOfIdx {
		out[k] = v
	}
	return out
}

func (encoder *DictionaryEncoder) GetIdx(key []byte) (int, bool) {
	idx, found := encoder.MapOfIdx[string(key)]
	return idx, found
}

func (encoder *DictionaryEncoder) GetKey(idx int) ([]byte, error) {
	if idx < 0 || idx >= len(encoder.keys) {
		return nil, errors.New("invalid key index")
	}
	return encoder.keys[idx], nil
}

func (encoder *DictionaryEncoder) AddToDict(key []byte) (int, bool) {
	strKey := string(key)
	if idx, found := encoder.MapOfIdx[strKey]; found {
		return idx, false
	}
	index := len(encoder.keys)
	keyCopy := make([]byte, len(key))
	copy(keyCopy, key)
	encoder.keys = append(encoder.keys, keyCopy)
	encoder.MapOfIdx[strKey] = index
	return index, true
}

func (encoder *DictionaryEncoder) RemoveFromDict(key []byte) bool {
	strKey := string(key)
	idx, found := encoder.MapOfIdx[strKey]
	if !found {
		return false
	}
	delete(encoder.MapOfIdx, strKey)
	encoder.keys = append(encoder.keys[:idx], encoder.keys[idx+1:]...)
	for i := idx; i < len(encoder.keys); i++ {
		encoder.MapOfIdx[string(encoder.keys[i])] = i
	}
	return true
}

func (encoder *DictionaryEncoder) RemoveAll() bool {
	encoder.keys = encoder.keys[:0]
	for i := range encoder.MapOfIdx {
		delete(encoder.MapOfIdx, i)
	}
	return true
}

func (encoder *DictionaryEncoder) Serialize() []byte {
	total := 0
	for _, k := range encoder.keys {
		total += binary.MaxVarintLen64 + len(k)
	}
	output := make([]byte, 0, total)
	for _, key := range encoder.keys {
		buff := make([]byte, binary.MaxVarintLen64)
		n := binary.PutUvarint(buff, uint64(len(key)))
		output = append(output, buff[:n]...)
		output = append(output, key...)
	}
	return output
}

func Deserialize(data []byte) (*DictionaryEncoder, error) {
	encoder := NewDictionaryEncoder()
	offset := 0
	for offset < len(data) {
		l, n := binary.Uvarint(data[offset:])
		if n == 0 {
			return nil, errors.New("deserialize: buffer too small for varint")
		}
		if n < 0 {
			return nil, errors.New("deserialize: varint overflow")
		}
		offset += n
		if offset+int(l) > len(data) {
			return nil, fmt.Errorf("deserialize: invalid key length %d at offset %d", l, offset)
		}
		key := data[offset : offset+int(l)]
		offset += int(l)
		if _, exists := encoder.MapOfIdx[string(key)]; exists {
			return nil, fmt.Errorf("deserialize: duplicate key %q", key)
		}
		encoder.MapOfIdx[string(key)] = len(encoder.keys)
		encoder.keys = append(encoder.keys, key)
	}
	return encoder, nil
}

// used for testing only
func LoadFromFile(dir, name string) (*DictionaryEncoder, error) {
	path := filepath.Join(dir, name)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NewDictionaryEncoder(), nil
	}
	if err != nil {
		return nil, err
	}
	return Deserialize(data)
}

// used for testing only
func (encoder *DictionaryEncoder) AppendLastToFile(dir, name string) error {
	if len(encoder.keys) == 0 {
		return errors.New("append last: dictionary is empty")
	}
	path := filepath.Join(dir, name)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	last := encoder.keys[len(encoder.keys)-1]
	buff := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(buff, uint64(len(last)))
	_, err = file.Write(buff[:n])
	if err != nil {
		return err
	}
	_, err = file.WriteString(string(last))
	return err
}

func (encoder *DictionaryEncoder) Size() int {
	return len(encoder.keys)
}
