package sstable

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"os"

	"github.com/ajromen/LSM-KV-Engine/internal/utils"
)

type IndexEntry struct {
	KeyLength uint64
	Key       []byte
	Offset    uint64
}

func (entry *IndexEntry) EncodeIndexEntry() []byte {
	buf := make([]byte, 0, 8+len(entry.Key)+8)
	buf = utils.AppendUvarint(buf, entry.KeyLength)
	buf = append(buf, entry.Key...)
	temp := make([]byte, 8)
	binary.LittleEndian.PutUint64(temp, entry.Offset)
	buf = append(buf, temp...)
	return buf
}

func DecodeIndexEntry(buf []byte) (*IndexEntry, int, error) {
	numRead := 0
	pos := 0
	keyLength, n := binary.Uvarint(buf[pos:])
	if n <= 0 {
		return nil, 0, errors.New("invalid index key length")
	}
	pos += n
	numRead += n
	key := make([]byte, int(keyLength))
	copy(key, buf[pos:pos+int(keyLength)])
	pos += int(keyLength)
	numRead += int(keyLength)
	offset := binary.LittleEndian.Uint64(buf[pos : pos+8])
	numRead += 8
	return &IndexEntry{
		KeyLength: keyLength,
		Key:       key,
		Offset:    offset,
	}, numRead, nil
}

type IndexBlock struct {
	Entries []IndexEntry
}

func NewIndexBlock() *IndexBlock {
	return &IndexBlock{
		Entries: make([]IndexEntry, 0),
	}
}

func (block *IndexBlock) AddEntry(entry IndexEntry) {
	block.Entries = append(block.Entries, entry)
}

func (block *IndexBlock) EncodeIndexBlock() []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(len(block.Entries)))
	for _, entry := range block.Entries {
		buf = append(buf, entry.EncodeIndexEntry()...)
	}
	crc := crc32.ChecksumIEEE(buf)
	temp := make([]byte, 4)
	binary.LittleEndian.PutUint32(temp, crc)
	buf = append(buf, temp...)
	return buf
}

func DecodeIndexBlock(buf []byte) (*IndexBlock, error) {
	crcPos := len(buf) - 4
	expected := binary.LittleEndian.Uint32(buf[crcPos : crcPos+4])
	crc := crc32.ChecksumIEEE(buf[:crcPos])
	if expected != crc {
		return nil, errors.New("crc mismatch")
	}
	numEntries := binary.LittleEndian.Uint32(buf[0:4])
	pos := 4
	entries := make([]IndexEntry, 0, numEntries)
	for i := 0; i < int(numEntries); i++ {
		entry, n, err := DecodeIndexEntry(buf[pos:crcPos])
		if err != nil {
			return nil, err
		}
		entries = append(entries, *entry)
		pos += n
	}
	return &IndexBlock{
		Entries: entries,
	}, nil
}

func (block *IndexBlock) FindBlock(key []byte) int {
	if len(block.Entries) == 0 {
		return -1
	}
	left := 0
	right := len(block.Entries) - 1
	result := -1
	for left <= right {
		mid := left + (right-left)/2
		cmp := bytes.Compare(key, block.Entries[mid].Key)
		if cmp < 0 {
			right = mid - 1
		} else {
			result = mid
			left = mid + 1
		}
	}
	return result
}

func (block *IndexBlock) Size() int {
	return len(block.EncodeIndexBlock())
}

func (block *IndexBlock) WriteToFile(file *os.File) (int, error) {
	data := block.EncodeIndexBlock()
	n, err := file.Write(data)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func ReadFromFile(file *os.File, offset uint64, size int) (*IndexBlock, error) {
	if _, err := file.Seek(int64(offset), 0); err != nil {
		return nil, err
	}
	data := make([]byte, size)
	if _, err := file.Read(data); err != nil {
		return nil, err
	}
	return DecodeIndexBlock(data)
}
