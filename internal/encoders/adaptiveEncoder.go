package encoders

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/fnv"
	"os"
)

type candidate struct {
	freq     uint64
	lastSeen uint64
	value    []byte
}

type dictEntry struct {
	value []byte
	id    uint32
}
type AdaptiveDict struct {
	windowSize uint64
	threshold  uint64
	entryCount uint64
	nextId     uint32
	candidates map[uint64][]*candidate
	dict       map[uint64][]*dictEntry
	idToValue  map[uint32][]byte
}

func NewAdaptiveDict(windowSize uint64, threshold uint64) *AdaptiveDict {
	return &AdaptiveDict{
		windowSize: windowSize,
		threshold:  threshold,
		entryCount: 0,
		nextId:     0,
		candidates: make(map[uint64][]*candidate),
		dict:       make(map[uint64][]*dictEntry),
		idToValue:  make(map[uint32][]byte),
	}
}

func (ad *AdaptiveDict) Reset() {
	ad.nextId = 0
	ad.candidates = make(map[uint64][]*candidate)
	ad.dict = make(map[uint64][]*dictEntry)
	ad.idToValue = make(map[uint32][]byte)
	ad.entryCount = 0
}

func hashValue(value []byte) uint64 {
	h := fnv.New64a()
	h.Write(value)
	return h.Sum64()
}

func hashToString(h uint64) string {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf[:], h)
	return string(buf[:])
}

func (ad *AdaptiveDict) EncodeValue(value []byte) (valueType byte, payload []byte) {
	ad.entryCount++
	h := hashValue(value)
	bucket, ok := ad.dict[h]
	if ok {
		for _, e := range bucket {
			if bytes.Equal(e.value, value) {
				return 1, binary.AppendUvarint(nil, uint64(e.id))
			}
		}
	}
	if len(value) <= 8 {
		return 0, value
	}
	if ad.windowSize > 0 {
		bucket := ad.candidates[h]
		nb := bucket[:0]
		for _, c := range bucket {
			if ad.entryCount-c.lastSeen <= ad.windowSize {
				nb = append(nb, c)
			}
		}
		if len(nb) == 0 {
			delete(ad.candidates, h)
		} else {
			ad.candidates[h] = nb
		}
	}
	var cand *candidate
	for _, candidate := range ad.candidates[h] {
		if bytes.Equal(value, candidate.value) {
			cand = candidate
			break
		}
	}
	if cand == nil {
		cand = &candidate{
			value:    value,
			freq:     1,
			lastSeen: ad.entryCount,
		}
		ad.candidates[h] = append(ad.candidates[h], cand)
	} else {
		cand.freq++
		cand.lastSeen = ad.entryCount
	}
	if cand.freq*uint64(len(cand.value)) >= ad.threshold {
		id := ad.nextId
		ad.nextId++
		ad.idToValue[id] = cand.value
		ad.dict[h] = append(ad.dict[h], &dictEntry{value: cand.value, id: id})
		b := ad.candidates[h]
		for i := range b {
			if b[i] == cand {
				ad.candidates[h] = append(b[:i], b[i+1:]...)
				break
			}
		}
		if len(b) == 0 {
			delete(ad.candidates, h)
		}
		return 1, binary.AppendUvarint(nil, uint64(id))
	}
	return 0, value
}

func (ad *AdaptiveDict) DecodeValue(valueType byte, payload []byte) ([]byte, error) {
	switch valueType {
	case 0:
		return payload, nil
	case 1:
		id, n := binary.Uvarint(payload)
		if n <= 0 {
			return nil, errors.New("invalid payload")
		}
		v, ok := ad.idToValue[uint32(id)]
		if !ok {
			return nil, errors.New("invalid id")
		}
		return v, nil
	default:
		return nil, errors.New("invalid type")
	}
}

func (ad *AdaptiveDict) WriteDictionary() ([]byte, error) {
	count := uint32(len(ad.idToValue))
	var res []byte
	temp := make([]byte, 4)
	binary.LittleEndian.PutUint32(temp, count)
	res = append(res, temp...)
	for id := uint32(0); id < count; id++ {
		v, ok := ad.idToValue[id]
		if !ok {
			return nil, errors.New("dictionary hole detected")
		}
		binary.LittleEndian.PutUint32(temp, uint32(len(v)))
		res = append(res, temp...)
		res = append(res, v...)
	}
	return res, nil
}

func (ad *AdaptiveDict) ReadDictionary(data []byte) error {
	ad.candidates = make(map[uint64][]*candidate)
	ad.dict = make(map[uint64][]*dictEntry)
	ad.idToValue = make(map[uint32][]byte)
	ad.nextId = 0
	if len(data) < 4 {
		return errors.New("data too short")
	}
	offset := 0
	count := binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4
	for i := uint32(0); i < count; i++ {
		if len(data) < offset+4 {
			return errors.New("unexpected end of data")
		}
		sz := binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4
		if len(data) < offset+int(sz) {
			return errors.New("data corrupted")
		}
		buf := data[offset : offset+int(sz)]
		offset += int(sz)
		h := hashValue(buf)
		ad.idToValue[i] = buf
		ad.dict[h] = append(ad.dict[h], &dictEntry{
			value: buf,
			id:    i,
		})
		ad.nextId = i + 1
	}
	return nil
}

func (ad *AdaptiveDict) WriteToFile(filepath string) error {
	data, err := ad.WriteDictionary()
	if err != nil {
		return err
	}
	return os.WriteFile(filepath, data, 0666)
}

func (ad *AdaptiveDict) ReadFromFile(filepath string) error {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return err
	}
	return ad.ReadDictionary(data)
}
