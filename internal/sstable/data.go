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

/*
┌─────────────────────────────────────────────────────────────────────┐
│                          DATA BLOCK                                 │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │  Record 1                                                      │ │
│  │    - timestamp (Uint128 as 2x uvarint: low, high)              │ │
│  │    - tombstone (1 byte: 0 = live, 1 = deleted)                 │ │
│  │    - key (delta encoded: shared_prefix_len + suffix_len + data │ │
│  │    - value_len (uvarint)                                       │ │
│  │    - value bytes                                               │ │
│  │                                                                │ │
│  │  Record 2                                                      │ │
│  │    - timestamp                                                 │ │
│  │    - tombstone                                                 │ │
│  │    - key (delta encoded)                                       │ │
│  │    - value_len                                                 │ │
│  │    - value bytes                                               │ │
│  │                                                                │ │
│  │  ...                                                           │ │
│  └────────────────────────────────────────────────────────────────┘ │
│                                                                     │
│  Restart Array (uint32 offsets into DATA SECTION)                   │
│    [restart_0][restart_1][restart_2]...                             │
│                                                                     │
│  Data Size      (uint32, little-endian)                             │
│  Restart Count  (uint32, little-endian) 							  |
|																      |
│  Padding ............................................               |
│                                                                     │
│  CRC32 Checksum (uint32, little-endian)                             │
└─────────────────────────────────────────────────────────────────────┘
NOTES:
Keys are delta-encoded: -> shared_prefix_len (uvarint) -> suffix_len (uvarint) -> suffix bytes
Every restart interval records, the full key is stored and a restart offset is written into the restart array.
The restart array allows faster binary search inside a block
CRC32 is calculated over the entire block except the last 4 bytes (CRC itself) we take padding into the calculation too
*/

// DATA BLOCK BUILDER BUILDS SSTABLE DATA BLOCK -> ENCODES KEYS USING DELTA ENCODING AND APPENDS RECORDS UNTIL THE BLOCK IS FULL
type DataBlockBuilder struct {
	encoder         *encoders.DeltaEncoderBytes // encoder used on given data
	data            []byte                      // raw data for block
	restartInterval int                         // number of keys between delta restart points
	recordCount     int                         // number of records added to block
	firstKey        []byte                      // first key in the block (for index)
}

func NewDataBlockBuilder(restartInterval, blockSize int) *DataBlockBuilder {
	return &DataBlockBuilder{
		encoder:         encoders.NewDeltaEncoderBytes(restartInterval),
		data:            make([]byte, 0, blockSize),
		restartInterval: restartInterval,
	}
}

func AppendUvarint128ToSlice2(buf []byte, v utils.Uint128) []byte {
	buf = binary.AppendUvarint(buf, v.Low)
	buf = binary.AppendUvarint(buf, v.High)
	return buf
}

// APPENDS A SINGLE RECORD TO THE CURRENT BLOCK -> RETURNS FALSE IF THE RECORD DOES NOT FIT IN THE REMAINING BLOCK CAPACITY

func (builder *DataBlockBuilder) AddRecord(record Record) bool {
	// if its the first record store its key as first key for block index
	if builder.recordCount == 0 {
		builder.firstKey = append([]byte(nil), record.Key...)
	}
	// rough size estimation to avoid overflow
	estimatedSize := len(record.Key) + len(record.Value) + 20
	if len(builder.data)+estimatedSize > cap(builder.data) && builder.recordCount > 0 {
		return false
	}
	keyOffset := uint32(len(builder.data))
	builder.data = AppendUvarint128ToSlice2(builder.data, record.Timestamp)
	if record.Tombstone {
		builder.data = append(builder.data, 1)
	} else {
		builder.data = append(builder.data, 0)
	}
	builder.data = builder.encoder.Encode(record.Key, keyOffset, builder.data)
	builder.data = utils.AppendUvarint(builder.data, uint64(len(record.Value)))
	builder.data = append(builder.data, record.Value...)
	builder.recordCount++
	return true
}

// FINALIZES THE DATA BLOCK BY APPENDING RESTART ARRAY, METADATA AND CRC ON ACTUAL DATA

func (builder *DataBlockBuilder) Finish(blockSize int) ([]byte, error) {
	if builder.recordCount == 0 {
		return nil, errors.New("empty block")
	}
	data := builder.data
	restarts := builder.encoder.RestartArray
	restartCount := uint32(len(restarts))
	block := make([]byte, blockSize)
	pos := 0
	// check if everything fits into the block
	if len(data)+int(restartCount)*4+4+4+4 > blockSize {
		return nil, errors.New("data for block too large")
	}
	// copy encoded data
	copy(block[pos:], data)
	pos += len(data)
	// append restart offsets
	for _, restart := range restarts {
		binary.LittleEndian.PutUint32(block[pos:], restart)
		pos += 4
	}
	// append data size
	binary.LittleEndian.PutUint32(block[pos:], uint32(len(data)))
	pos += 4
	// append restart count
	binary.LittleEndian.PutUint32(block[pos:], restartCount)
	pos += 4
	// append CRC
	crc := crc32.ChecksumIEEE(block[:blockSize-4])
	binary.LittleEndian.PutUint32(block[blockSize-4:], crc)
	return block, nil
}

// CLEARS THE BUILDER SO IT CAN BE REUSED FOR ANOTHER BLOCK

func (builder *DataBlockBuilder) Reset() {
	builder.data = builder.data[:0]
	builder.encoder.Reset()
	builder.recordCount = 0
	builder.firstKey = nil
}

func (builder *DataBlockBuilder) FirstKey() []byte {
	return builder.firstKey
}

func (builder *DataBlockBuilder) RecordCount() int {
	return builder.recordCount
}

// DATA BLOCK READER READS AND DECODES RECORDS FROM A SINGLE DATA BLOCK
type DataBlockReader struct {
	data         []byte                      // actual data in block
	decoder      *encoders.DeltaEncoderBytes // delta encoder
	restartArray []uint32                    // restart points for binary search
	pos          int                         // current read position
	dataSize     int                         // size of the actual data
}

// VERIFIES CRC AND INITIALIZES A BLOCK READER FOR GIVEN BLOCK

func NewDataBlockReader(block []byte) (*DataBlockReader, error) {
	if len(block) < 12 {
		return nil, errors.New("block too small")
	}
	// verify CRC
	expectedCRC := binary.LittleEndian.Uint32(block[len(block)-4:])
	actualCRC := crc32.ChecksumIEEE(block[:len(block)-4])
	if expectedCRC != actualCRC {
		return nil, errors.New("CRC mismatch")
	}
	// Locate metadata (skip padding)
	endData := len(block) - 5
	for endData > 0 && block[endData-4] == 0 {
		endData--
	}
	//read number of restarts in restart array
	restartCount := binary.LittleEndian.Uint32(block[endData-4 : endData])
	// read the size of actual data
	dataSize := binary.LittleEndian.Uint32(block[endData-8 : endData-4])
	// read restart array
	restartArray := make([]uint32, restartCount)
	startRestart := int(dataSize)
	for i := 0; i < int(restartCount); i++ {
		offset := startRestart + i*4
		restartArray[i] = binary.LittleEndian.Uint32(block[offset : offset+4])
	}
	// take actual data from block
	data := block[:dataSize]
	return &DataBlockReader{
		data:         data,
		decoder:      encoders.NewDeltaEncoderBytes(2),
		restartArray: restartArray,
		pos:          0,
		dataSize:     int(dataSize),
	}, nil
}

func (r *DataBlockReader) Restart() {
	r.pos = 0
}

// READS AND DECODES THE NEXT RECORD FROM THE BLOCK

func (r *DataBlockReader) ReadRecord() (*Record, error) {
	if r.pos >= r.dataSize {
		return nil, errors.New("out of data")
	}
	// read timestamp
	low, n1 := binary.Uvarint(r.data[r.pos:])
	if n1 <= 0 {
		return nil, errors.New("invalid low uint64")
	}
	r.pos += n1
	high, n2 := binary.Uvarint(r.data[r.pos:])
	if n2 <= 0 {
		return nil, errors.New("invalid high uint64")
	}
	r.pos += n2
	timestamp := utils.Uint128{
		High: high,
		Low:  low,
	}
	if r.pos >= r.dataSize {
		return nil, errors.New("unexpected end")
	}
	// read tombstone
	tombstone := r.data[r.pos] == 1
	r.pos++
	// read key
	key, err := r.decoder.Decode(r.data, &r.pos)
	if err != nil {
		return nil, err
	}
	// read value len
	valLen, n := binary.Uvarint(r.data[r.pos:])
	if n <= 0 {
		return nil, errors.New("invalid value size")
	}
	r.pos += n
	if r.pos+int(valLen) > len(r.data) {
		return nil, errors.New("value exceeds block bounds")
	}
	// read value
	value := append([]byte(nil), r.data[r.pos:r.pos+int(valLen)]...)
	r.pos += int(valLen)
	return &Record{
		Timestamp: timestamp,
		Tombstone: tombstone,
		Key:       key,
		Value:     value,
	}, nil
}

// positions the reader on the nth restart key in actual data -> allows faster binary search
func (r *DataBlockReader) SeekToRestart(idx int) error {
	if idx < 0 || idx >= len(r.restartArray) {
		return errors.New("invalid restart index")
	}
	r.pos = int(r.restartArray[idx])
	r.decoder.Reset()
	return nil
}

func (r *DataBlockReader) Close() error {
	return nil
}

func (r *DataBlockReader) Data() []byte {
	return r.data
}

func (r *DataBlockReader) DataSize() int {
	return r.dataSize
}

func (r *DataBlockReader) HasNext() bool {
	return r.pos < r.dataSize
}

// READS AND DECODES THE NEXT RECORD FROM THE BLOCK BUT ALSO RETURNS shared prefix len / shared sufix len / suffix

func (r *DataBlockReader) ReadRecordWithMeta() (*Record, uint64, uint64, []byte, error) {
	if r.pos >= len(r.data) {
		return nil, 0, 0, nil, errors.New("out of data")
	}
	low, n1 := binary.Uvarint(r.data[r.pos:])
	if n1 <= 0 {
		return nil, 0, 0, nil, errors.New("out of data")
	}
	r.pos += n1
	high, n2 := binary.Uvarint(r.data[r.pos:])
	if n2 <= 0 {
		return nil, 0, 0, nil, errors.New("out of data")
	}
	r.pos += n2
	timestamp := utils.Uint128{
		High: high,
		Low:  low,
	}
	tombstone := r.data[r.pos] == 1
	r.pos++
	shared, suffixLen, suffix, key, err := r.decoder.DecodeWithMeta(r.data, &r.pos)
	if err != nil {
		return nil, 0, 0, nil, err
	}
	valueLen, n := binary.Uvarint(r.data[r.pos:])
	if n <= 0 {
		return nil, 0, 0, nil, errors.New("invalid value size")
	}
	r.pos += n
	value := make([]byte, valueLen)
	copy(value, r.data[r.pos:r.pos+int(valueLen)])
	r.pos += int(valueLen)
	return &Record{
		Timestamp: timestamp,
		Tombstone: tombstone,
		Key:       key,
		Value:     value,
	}, shared, suffixLen, suffix, nil
}

// DATA BLOCK ITERATOR PROVIDES SEQUENTIAL AND SEEK BASED ITERATION OVER RECORDS INSIDE A SINGLE SSTABLE DATA BLOCK
type DataBlockIterator struct {
	reader  *DataBlockReader // underlying block reader
	current *Record          // record block iterator is currently pointing to
	valid   bool             // is iterator in valid state
}

func (i *DataBlockIterator) Reader() *DataBlockReader {
	return i.reader
}

func (i *DataBlockIterator) Current() *Record {
	return i.current
}

func (i *DataBlockIterator) Valid() bool {
	return i.valid
}

func (i *DataBlockIterator) Close() error {
	return nil
}

// CREATES A NEW ITERATOR OVER A RAW DATA BLOCK

func NewDataBlockIterator(block []byte) (*DataBlockIterator, error) {
	reader, err := NewDataBlockReader(block)
	if err != nil {
		return nil, err
	}
	// IMPORTANT : ITERATOR STATIS IN INVALID STATE UNTIL REWIND() OR SEEK() IS CALLED -> best practice iterator := NewIterator -> iterator.Rewind()
	iterator := &DataBlockIterator{
		reader:  reader,
		current: nil,
		valid:   false,
	}
	return iterator, nil
}

// POSITIONS THE ITERATOR AT THE FIRST RECOND IN THE BLOCK

func (i *DataBlockIterator) Rewind() error {
	if err := i.reader.SeekToRestart(0); err != nil {
		i.valid = false
		return err
	}
	record, err := i.reader.ReadRecord()
	if err != nil {
		i.valid = false
		return err
	}
	i.current = record
	i.valid = true
	return nil
}

// ADVANCES THE ITERATOR TO THE NEXT RECORD IN THE BLOCK

func (i *DataBlockIterator) Next() error {
	if !i.valid {
		return nil
	}
	if !i.reader.HasNext() {
		i.valid = false
		return nil
	}
	record, err := i.reader.ReadRecord()
	if err != nil {
		i.valid = false
		return err
	}
	i.current = record
	i.valid = true
	return nil
}

// MOVES THE ITERATOR TO THE FIRST RECORD WITH KEY >= TARGET -> binary search over restart points, then scans linearly

func (i *DataBlockIterator) Seek(target []byte) error {
	restarts := i.reader.restartArray
	left := 0
	right := len(restarts) - 1
	best := 0
	for left <= right {
		mid := left + (right-left)/2
		r := i.reader
		r.SeekToRestart(mid)
		record, err := i.reader.ReadRecord()
		if err != nil {
			i.valid = false
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
	i.reader.SeekToRestart(best)
	for i.reader.HasNext() {
		record, err := i.reader.ReadRecord()
		if err != nil {
			i.valid = false
			return err
		}
		cmp := bytes.Compare(record.Key, target)
		if cmp >= 0 {
			i.current = record
			i.valid = true
			return nil
		}
	}
	i.valid = false
	return nil
}

func (i *DataBlockIterator) Key() []byte {
	return i.current.Key
}

func (i *DataBlockIterator) Value() []byte {
	return i.current.Value
}

func (i *DataBlockIterator) Timestamp() utils.Uint128 {
	return i.current.Timestamp
}

func (i *DataBlockIterator) Tombstone() bool {
	return i.current.Tombstone
}

// MERGE ITERATOR MERGES TWO SORTED DATABLOCKITERATORS INTO ONE LOGICAL SORTED STREAM
type MergeIterator struct {
	iterator1 *DataBlockIterator // first input iterator
	iterator2 *DataBlockIterator // second input iterator
	Current   *DataBlockIterator // iterator that currently holds the smallest key
	valid     bool               // whether any iterator has valid state
}

func NewMergeIterator(iterator1, iterator2 *DataBlockIterator) *MergeIterator {
	iterator := &MergeIterator{
		iterator1: iterator1,
		iterator2: iterator2,
		valid:     iterator1.Valid() || iterator2.Valid(),
	}
	if iterator1 != nil && iterator1.Valid() {
		iterator.valid = true
	}
	if iterator2 != nil && iterator2.Valid() {
		iterator.valid = true
	}
	iterator.selectCurrent()
	return iterator
}

// CHOOSES WHICH UNDELYING ITERATOR CURRENTLY POINTS TO THE SMALLEST KEY (OR NEWEST VERSION OF SAME KEY)

func (iterator *MergeIterator) selectCurrent() {
	if !iterator.iterator1.Valid() && !iterator.iterator2.Valid() {
		iterator.valid = false
		return
	}
	if !iterator.iterator1.Valid() {
		iterator.Current = iterator.iterator2
		return
	}
	if !iterator.iterator2.Valid() {
		iterator.Current = iterator.iterator1
		return
	}
	if iterator.iterator1 == nil || !iterator.iterator1.Valid() {
		if iterator.iterator2 == nil || !iterator.iterator2.Valid() {
			iterator.valid = false
			return
		}
		iterator.Current = iterator.iterator2
		return
	}
	cmp := bytes.Compare(iterator.iterator1.Key(), iterator.iterator2.Key())
	if cmp < 0 {
		iterator.Current = iterator.iterator1
	} else if cmp > 0 {
		iterator.Current = iterator.iterator2
	} else {
		if utils.Uint128GE(iterator.iterator1.Timestamp(), iterator.iterator2.Timestamp()) {
			iterator.Current = iterator.iterator1
		} else {
			iterator.Current = iterator.iterator2
		}
	}
}

// MOVES THE MERGE ITERATOR FORWARD

func (iterator *MergeIterator) Advance() error {
	if !iterator.valid {
		return nil
	}
	if !iterator.iterator1.Valid() && !iterator.iterator2.Valid() {
		iterator.valid = false
		return nil
	}
	if !iterator.iterator1.Valid() {
		if err := iterator.iterator2.Next(); err != nil {
			return err
		}
	} else if !iterator.iterator2.Valid() {
		if err := iterator.iterator1.Next(); err != nil {
			return err
		}
	} else {
		cmp := bytes.Compare(iterator.iterator1.Key(), iterator.iterator2.Key())
		if cmp == 0 {
			if err := iterator.iterator1.Next(); err != nil {
				return err
			}
			if err := iterator.iterator2.Next(); err != nil {
				return err
			}
		} else if iterator.Current == iterator.iterator1 {
			if err := iterator.iterator1.Next(); err != nil {
				return err
			}
		} else {
			if err := iterator.iterator2.Next(); err != nil {
				return err
			}
		}
	}
	iterator.selectCurrent()
	return nil
}

func (iterator *MergeIterator) Close() error {
	iterator.iterator1.Close()
	iterator.iterator2.Close()
	return nil
}

func (iterator *MergeIterator) Valid() bool {
	return iterator.valid
}

func (iterator *MergeIterator) Next() error {
	return iterator.Advance()
}

func (iterator *MergeIterator) Key() []byte {
	if iterator.Current == nil {
		return nil
	}
	return iterator.Current.Key()
}

func (iterator *MergeIterator) Value() []byte {
	return iterator.Current.Value()
}

func (iterator *MergeIterator) Timestamp() utils.Uint128 {
	return iterator.Current.Timestamp()
}

func (iterator *MergeIterator) Tombstone() bool {
	return iterator.Current.Tombstone()
}
