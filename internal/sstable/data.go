package sstable

import (
	""
	"encoding/binary"
	"github.com/ajromen/LSM-KV-Engine/internal/compressor"
	"github.com/ajromen/LSM-KV-Engine/internal/utils"
	"os"
)

type DataRecord struct {
	CRC       uint32
	Timestamp uint64
	Tombstone bool
	Key       []byte
	Value     []byte
}

type DataFile struct {
	Filename    string
	startOffset int64
	Size        int64
}

func NewDataFile(filename string) (*DataFile, error) {
	_, err := os.Create(filename)
	if err != nil {
		return nil, err
	}
	return &DataFile{
		Filename:    filename,
		startOffset: 0,
		Size:        0,
	}, nil
}

func (df *DataFile) writeRecord(file *os.File, record *DataRecord, compressorDict *compressor.CompressorDict) error {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, record.CRC)
	_, err := file.Write(buf)
	if err != nil {
		return err
	}
	err = utils.WriteUvarint(file, uint64(record.Timestamp))
	if err != nil {
		return err
	}
	if record.Tombstone {
		_, err = file.Write([]byte{1})
		if err != nil {
			return err
		}
	} else {
		_, err = file.Write([]byte{0})
		if err != nil {
			return err
		}
	}
	if compressorDict == nil {
		err = utils.WriteUvarint(file, uint64(len(record.Key)))
		if err != nil {
			return err
		}
	}
	if !record.Tombstone {
		err = utils.WriteUvarint(file, uint64(len(record.Value)))
		if err != nil {
			return err
		}
	}
	if compressorDict == nil {
		_, err = file.Write(record.Key)
		if err != nil {
			return err
		}
	} else {
		idx, _ := compressorDict.GetIdx(string(record.Key))
		err = utils.WriteUvarint(file, uint64(idx))
		if err != nil {
			return err
		}
	}
	if !record.Tombstone {
		_, err = file.Write(record.Value)
		if err != nil {
			return err
		}
	}
	return nil
}

func (df *DataFile) readRecord(file *os.File, compressorDict *compressor.CompressorDict) (*DataRecord, error) {
	buf := make([]byte, 4)
	r, err := file.Read(buf)
	if r != 4 {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	CRC := binary.LittleEndian.Uint32(buf)
	timestamp, err := utils.ReadUvarint(file)
	if err != nil {
		return nil, err
	}
	buf = make([]byte, 1)
	r, err = file.Read(buf)
	if r != 1 {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	tomb := buf[0]
	tombstone := false
	if tomb == 1 {
		tombstone = true
	}
	var keySize uint64
	if compressorDict == nil {
		keySize, err = utils.ReadUvarint(file)
		if err != nil {
			return nil, err
		}
	}
	var valueSize uint64
	if !tombstone {
		valueSize, err = utils.ReadUvarint(file)
		if err != nil {
			return nil, err
		}
	}
	var key []byte
	if compressorDict == nil {
		key = make([]byte, keySize)
		_, err = file.Read(key)
		if err != nil {
			return nil, err
		}
	} else {
		keyIndex, err := utils.ReadUvarint(file)
		if err != nil {
			return nil, err
		}
		keyStr, _ := compressorDict.GetKey(int(keyIndex))
		key = []byte(keyStr)
	}
	var value []byte
	if !tombstone {
		value = make([]byte, valueSize)
		_, err = file.Read(value)
		if err != nil {
			return nil, err
		}
	}
	record := &DataRecord{
		CRC:       CRC,
		Timestamp: timestamp,
		Tombstone: tombstone,
		Key:       key,
		Value:     value,
	}
	return record, nil
}
