package sstable

import (
	"encoding/binary"
	"errors"
	"sort"
)

type MetadataFieldID uint16

const (
	FieldMinKey          MetadataFieldID = 1
	FieldMaxKey          MetadataFieldID = 2
	FieldMinTimestamp    MetadataFieldID = 3
	FieldMaxTimestamp    MetadataFieldID = 4
	FieldBlockSize       MetadataFieldID = 5
	FieldNumDataBlocks   MetadataFieldID = 6
	FieldTotalRecords    MetadataFieldID = 7
	FieldRestartInterval MetadataFieldID = 8
	FieldCompressionType MetadataFieldID = 10
	FieldMergeStrategy   MetadataFieldID = 11
	FieldMinKeyLength    MetadataFieldID = 12
	FieldMaxKeyLength    MetadataFieldID = 13
	FieldUserMetaStart   MetadataFieldID = 1000 // copied from leveldb -> future extension space
)

type Metadata struct {
	Fields map[MetadataFieldID][]byte
}

func (m *Metadata) Encode() []byte {
	var buf []byte
	ids := make([]int, 0, len(m.Fields))
	for id := range m.Fields {
		ids = append(ids, int(id))
	}
	sort.Ints(ids)
	for _, iid := range ids {
		id := MetadataFieldID(iid)
		val := m.Fields[id]
		tmp := make([]byte, 2+4+len(val))
		binary.LittleEndian.PutUint16(tmp[0:], uint16(id))
		binary.LittleEndian.PutUint32(tmp[2:], uint32(len(val)))
		copy(tmp[6:], val)
		buf = append(buf, tmp...)
	}
	return buf
}

func Decode(buf []byte) (*Metadata, error) {
	m := &Metadata{
		Fields: make(map[MetadataFieldID][]byte),
	}
	pos := 0
	for pos < len(buf) {
		if pos+6 > len(buf) {
			return nil, errors.New("corrupt metadata")
		}
		id := MetadataFieldID(binary.LittleEndian.Uint16(buf[pos:]))
		pos += 2
		l := binary.LittleEndian.Uint32(buf[pos:])
		pos += 4
		if pos+int(l) > len(buf) {
			return nil, errors.New("invalid field length")
		}
		val := make([]byte, l)
		copy(val, buf[pos:pos+int(l)])
		pos += int(l)
		m.Fields[id] = val
	}
	return m, nil
}

func (m *Metadata) GetByte(id MetadataFieldID) (byte, bool) {
	v, ok := m.Fields[id]
	if !ok || len(v) != 1 {
		return 0, false
	}
	return v[0], true
}

func (m *Metadata) GetBytes(id MetadataFieldID) []byte {
	return m.Fields[id]
}

func (m *Metadata) GetUint64(id MetadataFieldID) (uint64, bool) {
	v, ok := m.Fields[id]
	if !ok || len(v) != 8 {
		return 0, false
	}
	return binary.LittleEndian.Uint64(v), true
}

func (m *Metadata) GetBool(id MetadataFieldID) (bool, bool) {
	v, ok := m.Fields[id]
	if !ok || len(v) != 1 {
		return false, false
	}
	return v[0] != 0, true
}

func (m *Metadata) GetString(id MetadataFieldID) (string, bool) {
	v, ok := m.Fields[id]
	if !ok {
		return "", false
	}
	return string(v), true
}

func (m *Metadata) SetByte(id MetadataFieldID, v byte) {
	m.Fields[id] = []byte{v}
}

func (m *Metadata) SetBytes(id MetadataFieldID, v []byte) {
	if v == nil {
		delete(m.Fields, id)
		return
	}
	m.Fields[id] = v
}

func (m *Metadata) SetUint64(id MetadataFieldID, v uint64) {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, v)
	m.Fields[id] = buf
}

func (m *Metadata) SetBool(id MetadataFieldID, v bool) {
	if v {
		m.Fields[id] = []byte{1}
	} else {
		m.Fields[id] = []byte{0}
	}
}

func (m *Metadata) SetString(id MetadataFieldID, s string) {
	m.Fields[id] = []byte(s)
}
