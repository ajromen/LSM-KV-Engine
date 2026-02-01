package utils

import (
	"encoding/binary"
	"os"
)

func WriteUvarint(file *os.File, value uint64) error {
	bytes := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(bytes, value)
	_, err := file.Write(bytes[:n])
	if err != nil {
		return err
	}
	return nil
}

func ReadUvarint(file *os.File) (uint64, error) {
	var result uint64
	var shift uint
	for {
		b := make([]byte, 1)
		_, err := file.Read(b)
		if err != nil {
			return 0, err
		}
		result |= uint64(b[0]&0x7F) << shift
		if b[0]&0x80 == 0 {
			break
		}
		shift += 7
	}
	return result, nil
}
