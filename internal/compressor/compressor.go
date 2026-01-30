package compressor

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type CompressorDict struct {
	keys     []string
	mapOfIdx map[string]int
}

func NewCompressorDict() *CompressorDict {
	return &CompressorDict{
		keys:     []string{},
		mapOfIdx: make(map[string]int),
	}
}

func (cd *CompressorDict) Reset() {
	cd.keys = []string{}
	cd.mapOfIdx = make(map[string]int)
}

func (cd *CompressorDict) GetIdx(key string) (int, bool) {
	idx, ok := cd.mapOfIdx[key]
	return idx, ok
}

func (cd *CompressorDict) Add(key string) (int, bool) {
	idx, ok := cd.GetIdx(key)
	if ok {
		return idx, false
	}
	index := len(cd.keys)
	cd.keys = append(cd.keys, key)
	cd.mapOfIdx[key] = index
	return index, true
}

func (cd *CompressorDict) GetKey(idx int) (string, bool) {
	if idx < 0 || idx >= len(cd.keys) {
		return "", false
	}
	return cd.keys[idx], true
}

func (cd *CompressorDict) Remove(key string) bool {
	idx, ok := cd.GetIdx(key)
	if !ok {
		return false
	}
	delete(cd.mapOfIdx, key)
	cd.keys = append(cd.keys[:idx], cd.keys[idx+1:]...)
	for i := idx; i < len(cd.keys); i++ {
		cd.mapOfIdx[cd.keys[i]] = i
	}
	return true
}

func (cd *CompressorDict) RemoveAll() {
	cd.keys = cd.keys[:0]
	for k := range cd.mapOfIdx {
		delete(cd.mapOfIdx, k)
	}
}

func (cd *CompressorDict) Size() int {
	return len(cd.keys)
}

func (cd *CompressorDict) Keys() []string {
	out := make([]string, len(cd.keys))
	copy(out, cd.keys)
	return out
}

func (cd *CompressorDict) MapOfIdx() map[string]int {
	out := make(map[string]int, len(cd.mapOfIdx))
	for k, v := range cd.mapOfIdx {
		out[k] = v
	}
	return out
}

func (cd *CompressorDict) Serialize() []byte {
	total := 0
	for _, k := range cd.keys {
		total += binary.MaxVarintLen64 + len(k)
	}
	output := make([]byte, 0, total)
	for _, key := range cd.keys {
		buff := make([]byte, binary.MaxVarintLen64)
		n := binary.PutUvarint(buff, uint64(len(key)))
		output = append(output, buff[:n]...)
		output = append(output, key...)
	}
	return output
}

func Deserialize(data []byte) (*CompressorDict, error) {
	cd := NewCompressorDict()
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
		if _, exists := cd.mapOfIdx[key]; exists {
			return nil, fmt.Errorf("deserialize: duplicate key %q", key)
		}
		cd.mapOfIdx[key] = len(cd.keys)
		cd.keys = append(cd.keys, key)
	}
	return cd, nil
}

func LoadFromFile(dir, name string) (*CompressorDict, error) {
	path := filepath.Join(dir, name)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NewCompressorDict(), nil
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

func (cd *CompressorDict) AppendLastToFile(dir, name string) error {
	if len(cd.keys) == 0 {
		return errors.New("append last: dictionary is empty")
	}
	path := filepath.Join(dir, name)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	last := cd.keys[len(cd.keys)-1]
	buff := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(buff, uint64(len(last)))
	_, err = file.Write(buff[:n])
	if err != nil {
		return err
	}
	_, err = file.WriteString(last)
	return err
}
