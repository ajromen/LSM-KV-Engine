package encoders

import (
	"encoding/binary"
	"errors"
)

type DeltaEncoderBytes struct {
	prevKey         []byte
	restartInterval int
	entryIndex      int
	restartArray    []uint32
}

func NewDeltaEncoderBytes(restartInterval int) *DeltaEncoderBytes {
	return &DeltaEncoderBytes{
		restartInterval: restartInterval,
	}
}

func (de *DeltaEncoderBytes) Reset() {
	de.prevKey = nil
	de.entryIndex = 0
	de.restartArray = de.restartArray[:0]
}

func (de *DeltaEncoderBytes) Encode(key []byte, blockOffset uint32, buf []byte) []byte {
	shared := 0
	if de.entryIndex == 0 || de.entryIndex == de.restartInterval {
		de.restartArray = append(de.restartArray, blockOffset)
		de.prevKey = nil
		de.entryIndex = 0
	} else {
		shared = sharedPrefixLen(de.prevKey, key)
	}
	suffix := key[shared:]
	buf = binary.AppendUvarint(buf, uint64(shared))
	buf = binary.AppendUvarint(buf, uint64(len(suffix)))
	buf = append(buf, suffix...)
	de.prevKey = append(de.prevKey[:0], key...)
	de.entryIndex++
	return buf
}

func (de *DeltaEncoderBytes) Decode(buf []byte, pos *int) ([]byte, error) {
	shared, n := binary.Uvarint(buf[*pos:])
	if n <= 0 {
		return nil, errors.New("invalid shared prefix")
	}
	*pos += n
	nonShared, n := binary.Uvarint(buf[*pos:])
	if n <= 0 {
		return nil, errors.New("invalid nonshared prefix")
	}
	*pos += n
	if int(shared) > len(de.prevKey) {
		return nil, errors.New("shared prefix exceeds previous key length")
	}
	if *pos+int(nonShared) > len(buf) {
		return nil, errors.New("buffer underflow")
	}
	key := make([]byte, int(shared+nonShared))
	copy(key, de.prevKey[:shared])
	copy(key[shared:], buf[*pos:*pos+int(nonShared)])
	*pos += int(nonShared)
	de.prevKey = key
	return key, nil
}

func sharedPrefixLen(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

func (de *DeltaEncoderBytes) WriteRestartArray(buf []byte) []byte {
	for _, off := range de.restartArray {
		buf = binary.LittleEndian.AppendUint32(buf, off)
	}
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(de.restartArray)))
	return buf
}

func (d *DeltaEncoderBytes) DecodeWithMeta(data []byte, pos *int) (shared uint64, suffixLen uint64, suffix []byte, key []byte, err error) {
	start := *pos
	shared, n := binary.Uvarint(data[*pos:])
	if n <= 0 {
		return 0, 0, nil, nil, errors.New("invalid shared len")
	}
	*pos += n
	suffixLen, n = binary.Uvarint(data[*pos:])
	if n <= 0 {
		return 0, 0, nil, nil, errors.New("invalid suffix len")
	}
	*pos += n
	if *pos+int(suffixLen) > len(data) {
		return 0, 0, nil, nil, errors.New("suffix overflow")
	}
	suffix = data[*pos : *pos+int(suffixLen)]
	*pos += int(suffixLen)
	prefix := d.prevKey[:shared]
	key = append(append([]byte{}, prefix...), suffix...)
	d.prevKey = key
	_ = start
	return
}

func (d *DeltaEncoderBytes) RestartArray() []uint32 {
	return d.restartArray
}

func (d *DeltaEncoderBytes) RestartInterval() int {
	return d.restartInterval
}
