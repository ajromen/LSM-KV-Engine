package utils

import (
	"encoding/binary"
	"errors"
	"os"
)

type Uint128 struct {
	High uint64
	Low  uint64
}

func Uint128GE(a, b Uint128) bool {
	if a.High > b.High {
		return true
	}
	if a.High < b.High {
		return false
	}
	return a.Low >= b.Low
}
func Uint128LT(a, b Uint128) bool {
	if a.High < b.High {
		return true
	}
	if a.High > b.High {
		return false
	}
	return a.Low < b.Low
}

// use this for writing uint32 || uint64
func WriteUvarint(file *os.File, value uint64) error {
	buf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(buf, value)
	_, err := file.Write(buf[:n])
	if err != nil {
		return err
	}
	return nil
}

// use this for reading uint32 || uint64
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

// use this for writing uint128 (timestamp 16B)
func WriteUvarint128(file *os.File, value Uint128) error {
	high := value.High
	low := value.Low
	for i := 0; i < 20; i++ {
		b := byte(low & 0x7F)
		low >>= 7
		if high != 0 {
			b |= byte(high<<((64-7*i)%64)) & 0x7F
		}
		if high != 0 || low != 0 {
			b |= 0x80
		}
		_, err := file.Write([]byte{b})
		if err != nil {
			return err
		}
		if high == 0 && low == 0 {
			break
		}
	}
	return nil
}

// use this for reading uint128 (timestamp 16B)
func ReadUvarint128(file *os.File) (Uint128, error) {
	var value Uint128
	var shift uint
	for i := 0; i < 20; i++ {
		b := make([]byte, 1)
		n, err := file.Read(b)
		if err != nil {
			return value, err
		}
		if n != 1 {
			return value, errors.New("error reading uint128")
		}
		if shift < 64 {
			value.Low |= uint64(b[0]&0x7F) << shift
		} else {
			value.High |= uint64(b[0]&0x7F) << (shift - 64)
		}
		if b[0]&0x80 == 0 {
			break
		}
		shift += 7
	}
	return value, nil
}

func AppendUvarint128ToSlice(buf []byte, value Uint128) []byte {
	high := value.High
	low := value.Low
	for i := 0; i < 20; i++ {
		b := byte(low & 0x7F)
		low >>= 7
		if high != 0 {
			b |= byte(high<<((64-7*uint(i))%64)) & 0x7F
		}
		if high != 0 || low != 0 {
			b |= 0x80
		}
		buf = append(buf, b)
		if high == 0 && low == 0 {
			break
		}
	}
	return buf
}

func AppendUvarint(buf []byte, value uint64) []byte {
	buf = binary.AppendUvarint(buf, value)
	return buf
}

func ReadUvarint128FromSlice(data []byte) (Uint128, int, error) {
	var value Uint128
	var shift uint
	var bytesRead int
	for i := 0; i < 20 && bytesRead < len(data); i++ {
		b := data[bytesRead]
		bytesRead++

		if shift < 64 {
			value.Low |= uint64(b&0x7F) << shift
		} else {
			value.High |= uint64(b&0x7F) << (shift - 64)
		}
		if b&0x80 == 0 {
			return value, bytesRead, nil
		}
		shift += 7
	}
	return value, 0, errors.New("invalid uint128 encoding")
}
