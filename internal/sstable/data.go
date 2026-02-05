package sstable

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"

	"github.com/ajromen/LSM-KV-Engine/internal/encoders"
	"github.com/ajromen/LSM-KV-Engine/internal/utils"
)

const (
	CompressionNone   byte = 0
	CompressionSnappy byte = 1
	CompressionZSTD   byte = 2
)

type DataBlockBuilder struct {
	encoder         *encoders.DeltaEncoderBytes
	buf             []byte
	restartInterval int
	recordCount     int
	blockSize       int
}

func NewDataBlockBuilder(restartInterval int, blockSize int) *DataBlockBuilder {
	return &DataBlockBuilder{
		encoder:         encoders.NewDeltaEncoderBytes(restartInterval),
		buf:             make([]byte, 0, blockSize),
		restartInterval: restartInterval,
		blockSize:       blockSize,
	}
}

func (d *DataBlockBuilder) AddRecord(record Record) bool {
	estimatedRestartSize := len(d.encoder.RestartArray)*4 + 4 + 1 + 4
	estimatedRecordSize := 20 + 1 + 10 + 10 + len(record.Key) + 10 + len(record.Value)
	if len(d.buf) > 0 && len(d.buf)+estimatedRecordSize+estimatedRestartSize > d.blockSize {
		return false
	}
	blockOffset := uint32(len(d.buf))
	d.buf = utils.AppendUvarint128ToSlice(d.buf, record.Timestamp)
	if record.Tombstone {
		d.buf = append(d.buf, 1)
	} else {
		d.buf = append(d.buf, 0)
	}
	d.buf = d.encoder.Encode(record.Key, blockOffset, d.buf)
	d.buf = utils.AppendUvarint(d.buf, uint64(len(record.Value)))
	d.buf = append(d.buf, record.Value...)
	d.recordCount++
	return true
}

func (d *DataBlockBuilder) Finish(compression byte) []byte {
	block := make([]byte, d.blockSize)
	data := d.buf
	switch compression {
	case CompressionNone:
		break
	case CompressionSnappy:
		// TODO : IMPLEMENT SNAPPY COMPRESSION
	case CompressionZSTD:
		// TODO : IMPLEMENT ZSTD COMPRESSION
	}
	pos := 0
	copy(block[pos:], data)
	pos += len(data)
	for _, off := range d.encoder.RestartArray {
		binary.LittleEndian.PutUint32(block[pos:], uint32(off))
		pos += 4
	}
	binary.LittleEndian.PutUint32(block[pos:], uint32(len(d.encoder.RestartArray)))
	pos += 4
	block[pos] = compression
	pos += 1
	dataEndPos := pos
	binary.LittleEndian.PutUint32(block[pos:], uint32(dataEndPos))
	pos += 4
	checksum := crc32.ChecksumIEEE(block[:pos])
	binary.LittleEndian.PutUint32(block[pos:], checksum)
	return block
}

func (d *DataBlockBuilder) Restart() {
	d.buf = d.buf[:0]
	d.encoder.Reset()
	d.recordCount = 0
}

func (d *DataBlockBuilder) Size() int {
	return len(d.buf)
}

func (d *DataBlockBuilder) Encoder() *encoders.DeltaEncoderBytes {
	return d.encoder
}

func (d *DataBlockBuilder) Buffer() []byte {
	return d.buf
}

func (d *DataBlockBuilder) RestartInterval() int {
	return d.restartInterval
}

func (d *DataBlockBuilder) RecordCount() int {
	return d.recordCount
}

func (d *DataBlockBuilder) BlockSize() int {
	return d.blockSize
}

func (d *DataBlockBuilder) Reset() {
	d.buf = d.buf[:0]
	d.encoder.Reset()
	d.recordCount = 0
}

type DataBlockReader struct {
	data         []byte
	decoder      *encoders.DeltaEncoderBytes
	restartArray []uint32
	compression  byte
	pos          int
	dataEnd      int
}

func NewDataBlockReader(data []byte) (*DataBlockReader, error) {
	if len(data) < 13 {
		return nil, errors.New("block too small")
	}
	pos := len(data) - 1
	for pos >= 0 && data[pos] == 0 {
		pos--
	}
	crcPos := pos - 3
	if crcPos < 0 {
		return nil, errors.New("invalid block format")
	}
	expectedCRC := binary.LittleEndian.Uint32(data[crcPos : crcPos+4])
	dataEndPos := crcPos - 4
	if dataEndPos < 0 {
		return nil, errors.New("invalid block format")
	}
	dataEnd := binary.LittleEndian.Uint32(data[dataEndPos : dataEndPos+4])
	actualCRC := crc32.ChecksumIEEE(data[:crcPos])
	if expectedCRC != actualCRC {
		return nil, errors.New("CRC mismatch")
	}
	pos = int(dataEnd) - 1
	compression := data[pos]
	pos--
	restartCount := binary.LittleEndian.Uint32(data[pos-3 : pos+1])
	pos -= 4
	restartArraySize := int(restartCount) * 4
	restartArrayPos := pos - restartArraySize + 1
	if restartArrayPos < 0 {
		return nil, errors.New("invalid block format")
	}
	restartArray := make([]uint32, restartCount)
	for i := 0; i < int(restartCount); i++ {
		offset := restartArrayPos + i*4
		restartArray[i] = binary.LittleEndian.Uint32(data[offset : offset+4])
	}
	actualDataEnd := restartArrayPos
	actualData := data[:actualDataEnd]
	switch compression {
	case CompressionNone:
		break
	case CompressionSnappy:
		// TODO : IMPLEMENT SNAPPY DECOMPRESSION
	case CompressionZSTD:
		// TODO : IMPLEMENT ZSTD DECOMPRESSION
	}
	return &DataBlockReader{
		data:         actualData,
		decoder:      encoders.NewDeltaEncoderBytes(0),
		restartArray: restartArray,
		compression:  compression,
		pos:          0,
		dataEnd:      actualDataEnd,
	}, nil
}

func (d *DataBlockReader) ReadRecord() (*Record, error) {
	if d.pos >= len(d.data) {
		return nil, errors.New("out of data")
	}
	timestamp, n, err := utils.ReadUvarint128FromSlice(d.data[d.pos:])
	if err != nil {
		return nil, err
	}
	d.pos += n
	if d.pos >= len(d.data) {
		return nil, errors.New("out of data")
	}
	tombstone := d.data[d.pos] == 1
	d.pos++
	key, err := d.decoder.Decode(d.data, &d.pos)
	if err != nil {
		return nil, err
	}
	valueLen, n := binary.Uvarint(d.data[d.pos:])
	if n <= 0 {
		return nil, errors.New("invalid value size")
	}
	d.pos += n
	if d.pos+int(valueLen) > len(d.data) {
		return nil, errors.New("value exceeds block bounds")
	}
	value := make([]byte, valueLen)
	copy(value, d.data[d.pos:d.pos+int(valueLen)])
	d.pos += int(valueLen)
	return &Record{
		Timestamp: timestamp,
		Tombstone: tombstone,
		Key:       key,
		Value:     value,
	}, nil
}

func (d *DataBlockReader) SeekToRestart(restartIndex int) error {
	if restartIndex < 0 || restartIndex >= len(d.restartArray) {
		return errors.New("invalid restart index")
	}
	d.pos = int(d.restartArray[restartIndex])
	d.decoder.Reset()
	return nil
}

func (d *DataBlockReader) HasNext() bool {
	return d.pos < len(d.data)
}

func (d *DataBlockReader) Next() (*Record, error) {
	if d.pos >= len(d.data) {
		return nil, errors.New("out of data")
	}
	timestamp, n, err := utils.ReadUvarint128FromSlice(d.data[d.pos:])
	if err != nil {
		return nil, err
	}
	d.pos += n
	if d.pos >= len(d.data) {
		return nil, errors.New("out of data")
	}
	tombstone := d.data[d.pos] == 1
	d.pos++
	key, err := d.decoder.Decode(d.data, &d.pos)
	if err != nil {
		return nil, err
	}
	valueLen, n := binary.Uvarint(d.data[d.pos:])
	if n <= 0 {
		return nil, errors.New("invalid value size")
	}
	d.pos += n
	if d.pos+int(valueLen) > len(d.data) {
		return nil, errors.New("value exceeds block bounds")
	}
	value := make([]byte, valueLen)
	copy(value, d.data[d.pos:d.pos+int(valueLen)])
	d.pos += int(valueLen)
	return &Record{
		Timestamp: timestamp,
		Tombstone: tombstone,
		Key:       key,
		Value:     value,
	}, nil
}

func (d *DataBlockReader) Data() []byte {
	return d.data
}

func (d *DataBlockReader) Decoder() *encoders.DeltaEncoderBytes {
	return d.decoder
}

func (d *DataBlockReader) RestartArray() []uint32 {
	return d.restartArray
}

func (d *DataBlockReader) Compression() string {
	switch d.compression {
	case CompressionNone:
		return "NONE"
	case CompressionSnappy:
		return "SNAPPY"
	case CompressionZSTD:
		return "ZSTD"
	default:
		return "UNKNOWN"
	}
}

func (d *DataBlockReader) Position() int {
	return d.pos
}

func (d *DataBlockReader) DataEnd() int {
	return d.dataEnd
}

func (d *DataBlockReader) Close() error {
	return nil
}

type DataBlockIterator struct {
	reader  *DataBlockReader
	current *Record
	valid   bool
}

func NewDataBlockIterator(data []byte) (*DataBlockIterator, error) {
	reader, err := NewDataBlockReader(data)
	if err != nil {
		return nil, err
	}
	iterator := &DataBlockIterator{
		reader:  reader,
		current: nil,
		valid:   false,
	}
	iterator.Rewind()
	return iterator, nil
}

func (iterator *DataBlockIterator) Rewind() {
	iterator.reader.SeekToRestart(0)
	rec, err := iterator.reader.ReadRecord()
	if err != nil {
		iterator.valid = false
		return
	}
	iterator.current = rec
	iterator.valid = true
}

func (iterator *DataBlockIterator) HasNext() bool {
	return iterator.valid && iterator.reader.HasNext()
}

func (iterator *DataBlockIterator) Next() error {
	if !iterator.valid {
		return nil
	}
	if !iterator.reader.HasNext() {
		iterator.valid = false
		return nil
	}
	record, err := iterator.reader.ReadRecord()
	if err != nil {
		iterator.valid = false
		return err
	}
	iterator.current = record
	iterator.valid = true
	return nil
}

func (iterator *DataBlockIterator) Seek(target []byte) error {
	restarts := iterator.reader.restartArray
	left := 0
	right := len(restarts) - 1
	best := 0
	for left <= right {
		mid := left + (right-left)/2
		iterator.reader.SeekToRestart(mid)
		record, err := iterator.reader.ReadRecord()
		if err != nil {
			iterator.valid = false
			return err
		}
		cmp := bytes.Compare(record.Key, target)
		if cmp < 0 {
			best = mid
			left = mid + 1
		} else {
			right = mid - 1
		}
	}
	iterator.reader.SeekToRestart(best)
	iterator.reader.decoder.Reset()
	for iterator.reader.HasNext() {
		record, err := iterator.reader.ReadRecord()
		if err != nil {
			iterator.valid = false
			return err
		}
		if bytes.Compare(record.Key, target) >= 0 {
			iterator.current = record
			iterator.valid = true
			return nil
		}
	}
	iterator.valid = false
	return nil
}

func (iterator *DataBlockIterator) Valid() bool {
	return iterator.valid
}

func (iterator *DataBlockIterator) Key() []byte {
	return iterator.current.Key
}

func (iterator *DataBlockIterator) Value() []byte {
	return iterator.current.Value
}

func (iterator *DataBlockIterator) Timestamp() utils.Uint128 {
	return iterator.current.Timestamp
}

func (iterator *DataBlockIterator) Tombstone() bool {
	return iterator.current.Tombstone
}

func (iterator *DataBlockIterator) Close() error {
	return nil
}
