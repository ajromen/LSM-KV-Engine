package sstable

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"

	"github.com/ajromen/LSM-KV-Engine/internal/data_structures"
	"github.com/ajromen/LSM-KV-Engine/internal/encoders"
	"github.com/ajromen/LSM-KV-Engine/internal/iterator"
	"github.com/ajromen/LSM-KV-Engine/internal/utils"
)

const (
	CompressionNone   byte = 0
	CompressionSnappy byte = 1
	CompressionZSTD   byte = 2
)

// not used

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

// DataBlockBuilder  BUILDS SSTABLE DATA BLOCK -> ENCODES KEYS USING DELTA ENCODING AND APPENDS RECORDS UNTIL THE BLOCK IS FULL
type DataBlockBuilder struct {
	encoder         encoders.Encoder // encoder used on given data
	data            []byte           // raw data for block
	restartInterval int              // number of keys between delta restart points
	recordCount     int              // number of records added to block
	firstKey        []byte           // first key in the block (for index)
}

func NewDataBlockBuilder(t byte, restartInterval, blockSize int) *DataBlockBuilder {
	return &DataBlockBuilder{
		encoder:         encoders.NewEncoder(t, restartInterval),
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
	if builder.recordCount == 0 {
		builder.firstKey = append([]byte(nil), record.Key...)
	}
	estimatedSize := len(record.Key)*2 + len(record.Value) + 32
	reserved := 12 + (builder.recordCount/builder.restartInterval+1)*4

	if len(builder.data)+estimatedSize+reserved > cap(builder.data) && builder.recordCount > 0 {
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
	restarts := builder.encoder.RestartArray()
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
	binary.LittleEndian.PutUint32(block[blockSize-12:], uint32(len(data)))
	pos += 4
	// append restart count
	binary.LittleEndian.PutUint32(block[blockSize-8:], restartCount)
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
	data         []byte           // actual data in block
	decoder      encoders.Encoder // delta encoder
	restartArray []uint32         // restart points for binary search
	pos          int              // current read position
	dataSize     int              // size of the actual data
}

// VERIFIES CRC AND INITIALIZES A BLOCK READER FOR GIVEN BLOCK

func NewDataBlockReader(block []byte, restartInterval int, encodingType byte) (*DataBlockReader, error) {
	if len(block) < 12 {
		return nil, errors.New("block too small")
	}

	// verify CRC
	expectedCRC := binary.LittleEndian.Uint32(block[len(block)-4:])
	actualCRC := crc32.ChecksumIEEE(block[:len(block)-4])
	if expectedCRC != actualCRC {
		return nil, errors.New("CRC mismatch")
	}

	dataSize := binary.LittleEndian.Uint32(block[len(block)-12 : len(block)-8])
	restartCount := binary.LittleEndian.Uint32(block[len(block)-8 : len(block)-4])

	restartArray := make([]uint32, restartCount)
	for i := 0; i < int(restartCount); i++ {
		offset := int(dataSize) + i*4
		restartArray[i] = binary.LittleEndian.Uint32(block[offset : offset+4])
	}

	return &DataBlockReader{
		data:         block[:dataSize],
		decoder:      encoders.NewEncoder(encodingType, restartInterval),
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

// DataBlockIteratorRaw iterates over a single data block at the raw level
// exposing all records including tombstones and older versions of same key
type DataBlockIteratorRaw struct {
	reader  *DataBlockReader
	current *Record
	valid   bool
}

// NewDataBlockIteratorRaw creates a new raw iterator for a single block
func NewDataBlockIteratorRaw(block []byte, restartInterval int, encodingType byte) (*DataBlockIteratorRaw, error) {
	reader, err := NewDataBlockReader(block, restartInterval, encodingType)
	if err != nil {
		return nil, err
	}
	it := &DataBlockIteratorRaw{reader: reader}
	return it, nil
}

func (it *DataBlockIteratorRaw) Valid() bool { return it.valid }

// SeekToFirst moves the iterator to the first record in a block by using restart array
func (it *DataBlockIteratorRaw) SeekToFirst() {
	if err := it.reader.SeekToRestart(0); err != nil {
		it.valid = false
		return
	}
	it.readNext()
}

// SeekToLast moves the iterator to the last record in a block -> O(n) - not used
func (it *DataBlockIteratorRaw) SeekToLast() {
	if err := it.reader.SeekToRestart(0); err != nil {
		it.valid = false
		return
	}
	var last *Record
	for it.reader.HasNext() {
		rec, err := it.reader.ReadRecord()
		if err != nil {
			break
		}
		last = rec
	}
	if last == nil {
		it.valid = false
		return
	}
	it.current = last
	it.valid = true
}

// Seek positions the iterator at the first record whose key is >= target Key -> steps:
// 1. Binary search over restart points
// 2. Linear scan from the chosen restart point
// 3. Stop at first record whose key is >= target.key
func (it *DataBlockIteratorRaw) Seek(target Record) {
	restarts := it.reader.restartArray
	left, right, best := 0, len(restarts)-1, 0
	for left <= right {
		mid := left + (right-left)/2
		if err := it.reader.SeekToRestart(mid); err != nil {
			it.valid = false
			return
		}
		rec, err := it.reader.ReadRecord()
		if err != nil {
			it.valid = false
			return
		}
		if bytes.Compare(rec.Key, target.Key) < 0 {
			best = mid
			left = mid + 1
		} else {
			right = mid - 1
		}
	}
	if err := it.reader.SeekToRestart(best); err != nil {
		it.valid = false
		return
	}
	for it.reader.HasNext() {
		rec, err := it.reader.ReadRecord()
		if err != nil {
			it.valid = false
			return
		}
		if bytes.Compare(rec.Key, target.Key) >= 0 {
			it.current = rec
			it.valid = true
			return
		}
	}
	it.valid = false
}

// Next moves iterator forward by using readNext helper function
func (it *DataBlockIteratorRaw) Next() {
	if !it.valid {
		return
	}
	it.readNext()
}

// Prev moves iterator backwards
func (it *DataBlockIteratorRaw) Prev() {
	if !it.valid || it.current == nil {
		return
	}
	targetKey := it.current.Key
	targetTS := it.current.Timestamp

	if err := it.reader.SeekToRestart(0); err != nil {
		it.valid = false
		return
	}
	var prev *Record
	for it.reader.HasNext() {
		rec, err := it.reader.ReadRecord()
		if err != nil {
			break
		}
		cmp := bytes.Compare(rec.Key, targetKey)
		if cmp < 0 {
			prev = rec
			continue
		}
		if cmp == 0 && utils.Uint128GE(targetTS, rec.Timestamp) {
			prev = rec
			continue
		}
		break
	}
	if prev == nil {
		it.valid = false
		return
	}
	it.current = prev
	it.valid = true
}

// Key and Value return current record
func (it *DataBlockIteratorRaw) Key() Record { return *it.current }

func (it *DataBlockIteratorRaw) Value() Record { return *it.current }

// readNext is a helper function:
// 1. Check whether next record exists
// 2. Read next record and set it as current record
// both are reader methods
func (it *DataBlockIteratorRaw) readNext() {
	if !it.reader.HasNext() {
		it.valid = false
		return
	}
	rec, err := it.reader.ReadRecord()
	if err != nil {
		it.valid = false
		return
	}
	it.current = rec
	it.valid = true
}

// DataBlockIterator iterates over single data block with filtering
// filtering -> skipping older versions of the same key and if the newest version of key is tombstone skip that key completely
// it uses raw data block iterator and its methods but has a special method called advance
type DataBlockIterator struct {
	rawIterator *DataBlockIteratorRaw
	current     *Record
	valid       bool
}

func NewDataBlockIterator(block []byte, restartInterval int, encodingType byte) (*DataBlockIterator, error) {
	raw, err := NewDataBlockIteratorRaw(block, restartInterval, encodingType)
	if err != nil {
		return nil, err
	}
	it := &DataBlockIterator{
		rawIterator: raw,
	}
	return it, nil
}

func (it *DataBlockIterator) Valid() bool { return it.valid }

func (it *DataBlockIterator) SeekToFirst() {
	it.rawIterator.SeekToFirst()
	it.advance(nil)
}

func (it *DataBlockIterator) SeekToLast() {
	it.rawIterator.SeekToFirst()
	var last *Record
	var prevKey []byte
	for it.rawIterator.Valid() {
		rec := it.rawIterator.Key()
		if !rec.Tombstone && !bytes.Equal(rec.Key, prevKey) {
			cp := rec
			last = &cp
			prevKey = rec.Key
		}
		it.rawIterator.Next()
	}
	if last == nil {
		it.valid = false
		return
	}
	it.current = last
	it.valid = true
}

func (it *DataBlockIterator) Seek(target Record) {
	it.rawIterator.Seek(target)
	it.advance(nil)
}

func (it *DataBlockIterator) Next() {
	if !it.valid {
		return
	}
	prevKey := it.current.Key
	it.rawIterator.Next()
	it.advance(prevKey)
}

func (it *DataBlockIterator) Prev() {
	panic("DataBlockIterator: Prev not implemented")
}

func (it *DataBlockIterator) Key() Record { return *it.current }

func (it *DataBlockIterator) Value() Record { return *it.current }

// advance moves the iterator to the next valid record
// it skips over record that match the previous key or are marked as tombstones
// prevKey: the last key returned by the iterator, used to avoid returning duplicates
func (it *DataBlockIterator) advance(prevKey []byte) {
	for it.rawIterator.Valid() {
		rec := it.rawIterator.Key()
		if prevKey != nil && bytes.Equal(rec.Key, prevKey) {
			it.rawIterator.Next()
			continue
		}
		if rec.Tombstone {
			prevKey = rec.Key
			it.rawIterator.Next()
			continue
		}
		cp := rec
		it.current = &cp
		it.valid = true
		return
	}
	it.current = nil
	it.valid = false
}

func recordComparator(a, b Record) int {
	if c := bytes.Compare(a.Key, b.Key); c != 0 {
		return c
	}
	if utils.Uint128GE(a.Timestamp, b.Timestamp) && !tsEqual(a.Timestamp, b.Timestamp) {
		return -1
	}
	if utils.Uint128GE(b.Timestamp, a.Timestamp) && !tsEqual(a.Timestamp, b.Timestamp) {
		return 1
	}
	return 0
}

func tsEqual(a, b utils.Uint128) bool {
	return a.Low == b.Low && a.High == b.High
}

// MergeIteratorRaw iterates over more than one data blocks at raw level
// exposing all records including tombstones and older versions of same key
// it uses merge structure to quickly determine the smallest entry in all raw iterators provided -> O(log n)
type MergeIteratorRaw struct {
	structure data_structures.MergeStructure[Record]
	current   *Record
	valid     bool
}

// NewMergeIteratorRaw creates a new raw merge iterator from given iterators over single data blocks
// it builds a merge structure from those iterators and sets a first current using syncFromWinner method
func NewMergeIteratorRaw(iters []*DataBlockIteratorRaw, mergeStructure byte) *MergeIteratorRaw {
	if len(iters) == 0 {
		return &MergeIteratorRaw{valid: false}
	}
	wrapped := make([]iterator.Iterator[Record], 0, len(iters))
	for _, it := range iters {
		if it != nil && it.Valid() {
			wrapped = append(wrapped, it)
		}
	}
	if len(wrapped) == 0 {
		return &MergeIteratorRaw{valid: false}
	}
	structure := data_structures.NewMergeStructure(mergeStructure, wrapped, recordComparator)
	m := &MergeIteratorRaw{structure: structure}
	m.syncFromWinner()
	return m
}

func (m *MergeIteratorRaw) Valid() bool { return m.valid }

// SeekToFirst moves the iterator on the record with the smallest key out of all data blocks (single data block iterators positioned on first record)
func (m *MergeIteratorRaw) SeekToFirst() {
	for _, it := range m.structure.Iterators() {
		it.SeekToFirst()
		m.structure.Update(it)
	}
	m.syncFromWinner()
}

// SeekToLast moves the iterator on the record with largest key out of all data blocks
func (m *MergeIteratorRaw) SeekToLast() {
	for _, it := range m.structure.Iterators() {
		it.SeekToLast()
		m.structure.Update(it)
	}
	m.syncFromWinner()
}

// Seek moves the iterator on the record whose key >= target key
// It does that by positioning all iterators of single data blocks on the record whose key >= target key
// Then using merge structure the record with smallest key of those is the first record whose key >= target key
func (m *MergeIteratorRaw) Seek(target Record) {
	for _, it := range m.structure.Iterators() {
		it.Seek(target)
		m.structure.Update(it)
	}
	m.syncFromWinner()
}

// Next moves iterator forward by taking the next merge structure winner
// Then it advances the iterator from which winner record is chosen
func (m *MergeIteratorRaw) Next() {
	if !m.valid {
		return
	}
	winner := m.structure.Winner()
	if winner == nil {
		m.valid = false
		return
	}
	m.structure.Advance(winner)
	m.syncFromWinner()
}

func (m *MergeIteratorRaw) Prev() {
	panic("MergeIteratorRaw: Prev not implemented")
}

func (m *MergeIteratorRaw) Key() Record   { return *m.current }
func (m *MergeIteratorRaw) Value() Record { return *m.current }

// syncFromWinner takes a winner from a merge structure (record with smallest key out of current records in iterators)
// sets current iterator record to that winner
// does not advance the iterator from which winner is chosen
func (m *MergeIteratorRaw) syncFromWinner() {
	winner := m.structure.Winner()
	if winner == nil || !winner.Valid() {
		m.current = nil
		m.valid = false
		return
	}
	rec := winner.Key()
	m.current = &rec
	m.valid = true
}

// MergeIterator iterates over more than one data blocks with filtering
// filtering -> skipping older versions of the same key and if the newest version of key is tombstone skip that key completely
// it uses raw merge iterator and its methods but has a special method called advance
type MergeIterator struct {
	raw     *MergeIteratorRaw
	current *Record
	valid   bool
}

func NewMergeIterator(iters []*DataBlockIteratorRaw, mergeStructure byte) *MergeIterator {
	raw := NewMergeIteratorRaw(iters, mergeStructure)
	m := &MergeIterator{raw: raw}
	m.advance(nil)
	return m
}

func (m *MergeIterator) Valid() bool { return m.valid }

func (m *MergeIterator) SeekToFirst() {
	m.raw.SeekToFirst()
	m.advance(nil)
}

func (m *MergeIterator) SeekToLast() {
	m.raw.SeekToFirst()
	var last *Record
	var prevKey []byte
	for m.raw.Valid() {
		rec := m.raw.Key()
		if !rec.Tombstone && !bytes.Equal(rec.Key, prevKey) {
			cp := rec
			last = &cp
			prevKey = rec.Key
		}
		m.skipCurrentKey()
	}
	if last == nil {
		m.valid = false
		return
	}
	m.current = last
	m.valid = true
}

func (m *MergeIterator) Seek(target Record) {
	m.raw.Seek(target)
	m.advance(nil)
}

func (m *MergeIterator) Next() {
	if !m.valid {
		return
	}
	prevKey := m.current.Key
	m.skipCurrentKey()
	m.advance(prevKey)
}

func (m *MergeIterator) Prev() {
	panic("MergeIterator: Prev not implemented")
}

func (m *MergeIterator) skipCurrentKey() {
	if !m.raw.Valid() {
		return
	}
	currentKey := append([]byte(nil), m.raw.Key().Key...)
	for m.raw.Valid() && bytes.Equal(m.raw.Key().Key, currentKey) {
		m.raw.Next()
	}
}

func (m *MergeIterator) Key() Record   { return *m.current }
func (m *MergeIterator) Value() Record { return *m.current }

func (m *MergeIterator) advance(prevKey []byte) {
	for m.raw.Valid() {
		rec := m.raw.Key()
		if prevKey != nil && bytes.Equal(rec.Key, prevKey) {
			m.skipCurrentKey()
			continue
		}
		if rec.Tombstone {
			prevKey = append([]byte(nil), rec.Key...)
			m.skipCurrentKey()
			continue
		}
		cp := rec
		m.current = &cp
		m.valid = true
		return
	}
	m.current = nil
	m.valid = false
}
