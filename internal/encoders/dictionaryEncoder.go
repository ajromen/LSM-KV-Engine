package encoders

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type DictionaryEncoder struct {
	keys     []string
	MapOfIdx map[string]int
}

func NewDictionaryEncoder() *DictionaryEncoder {
	return &DictionaryEncoder{
		keys:     make([]string, 0),
		MapOfIdx: make(map[string]int),
	}
}

func (encoder *DictionaryEncoder) GetKeys() []string {
	out := make([]string, len(encoder.keys))
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

func (encoder *DictionaryEncoder) GetIdx(key string) (int, bool) {
	idx, found := encoder.MapOfIdx[key]
	return idx, found
}

func (encoder *DictionaryEncoder) GetKey(idx int) (string, error) {
	if idx < 0 || idx >= len(encoder.keys) {
		return "", errors.New("invalid key index")
	}
	return encoder.keys[idx], nil
}

func (encoder *DictionaryEncoder) AddToDict(key string) (int, bool) {
	idx, found := encoder.GetIdx(key)
	if found {
		return idx, false
	}
	index := len(encoder.keys)
	encoder.keys = append(encoder.keys, key)
	encoder.MapOfIdx[key] = index
	return index, true
}

func (encoder *DictionaryEncoder) RemoveFromDict(key string) bool {
	idx, found := encoder.GetIdx(key)
	if !found {
		return false
	}
	delete(encoder.MapOfIdx, key)
	encoder.keys = append(encoder.keys[:idx], encoder.keys[idx+1:]...)
	for i := idx; i < len(encoder.keys); i++ {
		encoder.MapOfIdx[encoder.keys[i]] = i
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
		key := string(data[offset : offset+int(l)])
		offset += int(l)
		if _, exists := encoder.MapOfIdx[key]; exists {
			return nil, fmt.Errorf("deserialize: duplicate key %q", key)
		}
		encoder.MapOfIdx[key] = len(encoder.keys)
		encoder.keys = append(encoder.keys, key)
	}
	return encoder, nil
}

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

func SaveToFile(dir, name string, data []byte) error {
	path := filepath.Join(dir, name)
	return os.WriteFile(path, data, 0644)
}

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
	_, err = file.WriteString(last)
	return err
}

func (encoder *DictionaryEncoder) Size() int {
	return len(encoder.keys)
}
