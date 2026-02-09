package sstable

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"os"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
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
	firstKey        []byte
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
	if d.recordCount == 0 {
		d.firstKey = append([]byte(nil), record.Key...)
	}
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

func (d *DataBlockBuilder) FirstKey() []byte {
	return d.firstKey
}

func (d *DataBlockBuilder) Reset() {
	d.buf = d.buf[:0]
	d.encoder.Reset()
	d.recordCount = 0
	d.firstKey = nil
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

func (d *DataBlockReader) ReadRecordWithMeta() (*Record, uint64, uint64, []byte, error) {
	if d.pos >= len(d.data) {
		return nil, 0, 0, nil, errors.New("out of data")
	}
	timestamp, n, err := utils.ReadUvarint128FromSlice(d.data[d.pos:])
	if err != nil {
		return nil, 0, 0, nil, err
	}
	d.pos += n
	tombstone := d.data[d.pos] == 1
	d.pos++
	shared, suffixLen, suffix, key, err := d.decoder.DecodeWithMeta(d.data, &d.pos)
	if err != nil {
		return nil, 0, 0, nil, err
	}
	valueLen, n := binary.Uvarint(d.data[d.pos:])
	if n <= 0 {
		return nil, 0, 0, nil, errors.New("invalid value size")
	}
	d.pos += n
	value := make([]byte, valueLen)
	copy(value, d.data[d.pos:d.pos+int(valueLen)])
	d.pos += int(valueLen)
	return &Record{
		Timestamp: timestamp,
		Tombstone: tombstone,
		Key:       key,
		Value:     value,
	}, shared, suffixLen, suffix, nil
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

func (iterator *DataBlockIterator) Rewind() error {
	if err := iterator.reader.SeekToRestart(0); err != nil {
		iterator.valid = false
		return err
	}
	rec, err := iterator.reader.ReadRecord()
	if err != nil {
		iterator.valid = false
		return err
	}
	iterator.current = rec
	iterator.valid = true
	return nil
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
		} else if cmp == 0 {
			iterator.current = record
			iterator.valid = true
			return nil
		} else {
			right = mid - 1
		}
	}
	iterator.reader.SeekToRestart(best)
	for iterator.reader.HasNext() {
		record, err := iterator.reader.ReadRecord()
		if err != nil {
			iterator.valid = false
			return err
		}
		cmp := bytes.Compare(record.Key, target)
		if cmp >= 0 {
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

type MergeIterator struct {
	iterator1 *DataBlockIterator
	iterator2 *DataBlockIterator
	current   *DataBlockIterator
	valid     bool
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

func (iterator *MergeIterator) selectCurrent() {
	if !iterator.iterator1.Valid() && !iterator.iterator2.Valid() {
		iterator.valid = false
		return
	}
	if !iterator.iterator1.Valid() {
		iterator.current = iterator.iterator2
		return
	}
	if !iterator.iterator2.Valid() {
		iterator.current = iterator.iterator1
		return
	}
	if iterator.iterator1 == nil || !iterator.iterator1.Valid() {
		if iterator.iterator2 == nil || !iterator.iterator2.Valid() {
			iterator.valid = false
			return
		}
		iterator.current = iterator.iterator2
		return
	}
	cmp := bytes.Compare(iterator.iterator1.Key(), iterator.iterator2.Key())
	if cmp < 0 {
		iterator.current = iterator.iterator1
	} else if cmp > 0 {
		iterator.current = iterator.iterator2
	} else {
		if utils.Uint128GE(iterator.iterator1.Timestamp(), iterator.iterator2.Timestamp()) {
			iterator.current = iterator.iterator1
		} else {
			iterator.current = iterator.iterator2
		}
	}
}

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
		} else if iterator.current == iterator.iterator1 {
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
	if iterator.current == nil {
		return nil
	}
	return iterator.current.Key()
}

func (iterator *MergeIterator) Value() []byte {
	return iterator.current.Value()
}

func (iterator *MergeIterator) Timestamp() utils.Uint128 {
	return iterator.current.Timestamp()
}

func (iterator *MergeIterator) Tombstone() bool {
	return iterator.current.Tombstone()
}

func VisualizeBlock(blockData []byte) error {
	reader, err := NewDataBlockReader(blockData)
	if err != nil {
		return err
	}
	fmt.Println("\n==========================================================================================")
	fmt.Println("VISUALIZING BLOCK")
	fmt.Println("==========================================================================================")
	fmt.Print("\nBLOCK METADATA\n")
	fmt.Printf("Block size: %d bytes\n", len(blockData))
	fmt.Printf("Data size: %d bytes\n", reader.dataEnd)
	fmt.Printf("Padding: %d bytes\n", len(blockData)-reader.dataEnd-17)
	fmt.Printf("Compression: %b \n", reader.compression)
	fmt.Printf("Restart points: %d\n", len(reader.restartArray))
	utilization := float64(reader.dataEnd) / float64(len(blockData)) * 100
	fmt.Printf("Utilization: %.2f%%\n", utilization)
	fmt.Printf("Restart points offset: \n")
	for i, offset := range reader.restartArray {
		fmt.Printf("[%d]=%d", i, offset)
		if (i+1)%8 == 0 && i < len(reader.restartArray)-1 {
			fmt.Printf("\n")
		}
	}
	fmt.Println()
	fmt.Print("\nRECORDS\n")
	fmt.Println("==========================================================================================")
	reader.SeekToRestart(0)
	recordIdx := 0
	restartIdx := 0
	nextRestart := uint32(0)
	if len(reader.restartArray) > 1 {
		nextRestart = reader.restartArray[1]
	} else {
		nextRestart = ^uint32(0)
	}
	for reader.HasNext() {
		startPos := reader.pos
		isRestart := false
		if restartIdx < len(reader.restartArray) && uint32(startPos) == reader.restartArray[restartIdx] {
			isRestart = true
			if restartIdx < len(reader.restartArray)-1 {
				nextRestart = reader.restartArray[restartIdx+1]
			} else {
				nextRestart = ^uint32(0)
			}
			restartIdx++
		}
		rec, err := reader.ReadRecord()
		if err != nil {
			break
		}
		endPos := reader.pos
		recordSize := endPos - startPos
		deleted := " "
		if rec.Tombstone {
			deleted = "[del]"
		}
		restartMarker := ""
		if isRestart {
			restartMarker = "[restart]"
		}
		valueDisplay := string(rec.Value)
		if len(valueDisplay) > 40 {
			valueDisplay = valueDisplay[:37] + "..."
		}
		keyDisplay := string(rec.Key)
		if len(keyDisplay) > 20 {
			keyDisplay = keyDisplay[:17] + "..."
		}
		fmt.Printf(" [%3d] %s %-20s → %-40s", recordIdx, deleted, keyDisplay, valueDisplay)
		fmt.Printf(" (%d bytes)%s\n", recordSize, restartMarker)
		recordIdx++
	}
	fmt.Println("==========================================================================================")
	fmt.Printf("Total Records: %d\n", recordIdx)
	fmt.Println(nextRestart)
	return nil
}

func VisualizeDataSegmentFromSSTable(filePath string, blockManager *block.BlockManager) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return err
	}
	fileSize := stat.Size()
	fmt.Println("==========================================================================================")
	fmt.Println("VISUALIZING DATA SEGMENT")
	fmt.Println("==========================================================================================")
	fmt.Println("FILE INFO")
	fmt.Printf("Path: %s\n", filePath)
	fmt.Printf("Size: %d bytes (%.2f KB)\n", fileSize, float64(fileSize)/1024)
	fmt.Printf("Block Size: %d bytes\n", blockManager.BlockSize())
	numBlocks := int(fileSize) / blockManager.BlockSize()
	fmt.Printf("Blocks: %d\n", numBlocks)
	for blockIdx := 0; blockIdx < numBlocks; blockIdx++ {
		fmt.Println("==========================================================================================")
		fmt.Printf("\nBLOCK %d (offset: %d bytes)\n", blockIdx, blockIdx*blockManager.BlockSize())
		fmt.Println("==========================================================================================")
		blockKey := block.BlockKey{
			FilePath: filePath,
			Offset:   uint32(blockIdx),
		}
		blockData, err := blockManager.Read(blockKey)
		if err != nil {
			fmt.Printf("Error reading block: %v\n", err)
			continue
		}
		reader, err := NewDataBlockReader(blockData)
		if err != nil {
			fmt.Printf("Error parsing block: %v\n", err)
			continue
		}
		recordCount := 0
		reader.SeekToRestart(0)
		for reader.HasNext() {
			_, err := reader.ReadRecord()
			if err != nil {
				break
			}
			recordCount++
		}
		fmt.Printf("Records: %d\n", recordCount)
		fmt.Printf("Data Size: %d bytes\n", reader.dataEnd)
		fmt.Printf("Restart Points: %d\n", len(reader.restartArray))
		fmt.Printf("Utilization: %.2f%%\n", float64(reader.dataEnd)/float64(blockManager.BlockSize())*100)
		reader.SeekToRestart(0)
		fmt.Printf("\nSample Records (first 5):\n")
		for i := 0; i < 5 && reader.HasNext(); i++ {
			rec, shared, suffixLen, suffix, err := reader.ReadRecordWithMeta()
			if err != nil {
				break
			}
			icon := ""
			if rec.Tombstone {
				icon = "[del]️"
			}
			keyDisplay := string(rec.Key)
			if len(keyDisplay) > 25 {
				keyDisplay = keyDisplay[:22] + "..."
			}
			valueDisplay := string(rec.Value)
			if len(valueDisplay) > 30 {
				valueDisplay = valueDisplay[:27] + "..."
			}
			fmt.Printf("[%d] %s %-25s → %s | shared=%d suffixLen=%d suffix=%q\n",
				i,
				icon,
				keyDisplay,
				valueDisplay,
				shared,
				suffixLen,
				string(suffix),
			)
		}
		if recordCount > 10 {
			fmt.Printf("... and %d more records\n", recordCount-5)
		}
	}
	fmt.Println("==========================================================================================")
	fmt.Printf("Summary:\n")
	fmt.Printf("Total Blocks: %d\n", numBlocks)
	fmt.Printf("File Size: %d bytes\n", fileSize)
	fmt.Printf("Avg/block: %.1f records\n", float64(fileSize)/float64(numBlocks)/70.0)
	fmt.Println("==========================================================================================")
	return nil
}
