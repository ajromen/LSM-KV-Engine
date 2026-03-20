package encoders

import (
	"encoding/binary"
	"errors"
)

type DeltaEncoderInt struct {
	prevKey         int
	restartInterval int
	entryIndex      int
	restartArray    []uint32
}

func NewDeltaEncoderInt(restartInterval int) *DeltaEncoderInt {
	return &DeltaEncoderInt{
		restartInterval: restartInterval,
	}
}

func (de *DeltaEncoderInt) Reset() {
	de.prevKey = 0
	de.entryIndex = 0
	de.restartArray = de.restartArray[:0]
}

func (de *DeltaEncoderInt) Encode(key int, blockOffset uint32, buf []byte) []byte {
	var delta int
	if de.entryIndex == 0 || de.entryIndex == de.restartInterval {
		de.restartArray = append(de.restartArray, blockOffset)
		delta = key
		de.prevKey = 0
		de.entryIndex = 0
	} else {
		delta = key - de.prevKey
	}
	buf = binary.AppendVarint(buf, int64(delta))
	de.prevKey = key
	de.entryIndex++
	return buf
}

func (de *DeltaEncoderInt) Decode(buf []byte, pos *int) (int, error) {
	delta, n := binary.Varint(buf[*pos:])
	if n <= 0 {
		return 0, errors.New("invalid delta int")
	}
	*pos += n
	var key int
	if de.entryIndex == 0 || de.entryIndex == de.restartInterval {
		key = int(delta)
		de.entryIndex = 0
	} else {
		key = de.prevKey + int(delta)
	}
	de.prevKey = key
	de.entryIndex++
	return key, nil
}

func (de *DeltaEncoderInt) WriteRestartArray(buf []byte) []byte {
	for _, off := range de.restartArray {
		buf = binary.LittleEndian.AppendUint32(buf, off)
	}
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(de.restartArray)))
	return buf
}

func (de *DeltaEncoderInt) RestartArray() []uint32 {
	return de.restartArray
}

func (de *DeltaEncoderInt) RestartInterval() int {
	return de.restartInterval
}
