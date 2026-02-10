package sstable

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
)

type TopLevelIndexEntry struct {
	FirstKey []byte
	Offset   uint64
	Size     uint32
}

func (entry *TopLevelIndexEntry) EncodedSize() int {
	keyLen := uint64(len(entry.FirstKey))
	buf := make([]byte, binary.MaxVarintLen64)
	varintLen := binary.PutUvarint(buf, keyLen)
	return varintLen + len(entry.FirstKey) + 8 + 4
}

func (entry *TopLevelIndexEntry) EncodeTo(buf []byte) int {
	keyLen := uint64(len(entry.FirstKey))
	pos := 0
	n := binary.PutUvarint(buf[pos:], keyLen)
	pos += n
	copy(buf[pos:], entry.FirstKey)
	pos += len(entry.FirstKey)
	binary.LittleEndian.PutUint64(buf[pos:], entry.Offset)
	pos += 8
	binary.LittleEndian.PutUint32(buf[pos:], entry.Size)
	pos += 4
	return pos
}

func (entry *TopLevelIndexEntry) EncodeIndexEntry() []byte {
	size := entry.EncodedSize()
	buf := make([]byte, size)
	entry.EncodeTo(buf)
	return buf
}

func DecodeTopLevelIndexEntry(buf []byte) (*TopLevelIndexEntry, int, error) {
	pos := 0
	keyLen, n := binary.Uvarint(buf[pos:])
	if n <= 0 {
		return nil, 0, errors.New("invalid top level index key")
	}
	pos += n
	if pos+int(keyLen)+12 > len(buf) {
		return nil, 0, errors.New("buffer too small")
	}
	key := make([]byte, keyLen)
	copy(key, buf[pos:pos+int(keyLen)])
	pos += int(keyLen)
	offset := binary.LittleEndian.Uint64(buf[pos:])
	pos += 8
	size := binary.LittleEndian.Uint32(buf[pos:])
	pos += 4
	return &TopLevelIndexEntry{
		FirstKey: key,
		Offset:   offset,
		Size:     size,
	}, pos, nil
}

type TopLevelIndex struct {
	Entries []TopLevelIndexEntry
}

func NewTopLevelIndex(config *config.Config) *TopLevelIndex {
	return &TopLevelIndex{
		Entries: make([]TopLevelIndexEntry, 0, config.SSTable.IndexSegment.IndexBlockSize),
	}
}

func (tli *TopLevelIndex) AddEntry(entry TopLevelIndexEntry) {
	tli.Entries = append(tli.Entries, entry)
}

func (tli *TopLevelIndex) EncodeTopLevelIndex() []byte {
	totalSize := tli.Size()
	buf := make([]byte, totalSize)
	pos := 0
	binary.LittleEndian.PutUint32(buf[pos:], uint32(len(tli.Entries)))
	pos += 4
	for i := range tli.Entries {
		n := tli.Entries[i].EncodeTo(buf[pos:])
		pos += n
	}
	crc := crc32.ChecksumIEEE(buf[:pos])
	binary.LittleEndian.PutUint32(buf[pos:], crc)
	return buf
}

func DecodeTopLevelIndex(buf []byte) (*TopLevelIndex, error) {
	if len(buf) < 8 {
		return nil, errors.New("Buffer too small")
	}
	crcPos := len(buf) - 4
	expected := binary.LittleEndian.Uint32(buf[crcPos:])
	crc := crc32.ChecksumIEEE(buf[:crcPos])
	if expected != crc {
		return nil, errors.New("crc mismatch")
	}
	numEntries := binary.LittleEndian.Uint32(buf[0:4])
	if numEntries == 0 {
		return &TopLevelIndex{Entries: []TopLevelIndexEntry{}}, nil
	}
	pos := 4
	entries := make([]TopLevelIndexEntry, 0, numEntries)
	for i := 0; i < int(numEntries); i++ {
		entry, n, err := DecodeTopLevelIndexEntry(buf[pos:crcPos])
		if err != nil {
			return nil, err
		}
		entries = append(entries, *entry)
		pos += n
	}
	return &TopLevelIndex{
		Entries: entries,
	}, nil
}

func (tli *TopLevelIndex) FindIndexBlock(key []byte) int {
	if len(tli.Entries) == 0 {
		return -1
	}
	if bytes.Compare(key, tli.Entries[0].FirstKey) < 0 {
		return -1
	}
	lastIdx := len(tli.Entries) - 1
	if bytes.Compare(key, tli.Entries[lastIdx].FirstKey) >= 0 {
		return lastIdx
	}
	left := 0
	right := lastIdx
	result := -1
	for left <= right {
		mid := left + (right-left)/2
		cmp := bytes.Compare(key, tli.Entries[mid].FirstKey)
		if cmp < 0 {
			right = mid - 1
		} else if cmp == 0 {
			return mid
		} else {
			result = mid
			left = mid + 1
		}
	}
	return result
}

func (tli *TopLevelIndex) Size() int {
	size := 4
	for i := range tli.Entries {
		size += tli.Entries[i].EncodedSize()
	}
	size += 4
	return size
}

func (tli *TopLevelIndex) WriteToFile(file *os.File) (int, error) {
	data := tli.EncodeTopLevelIndex()
	n, err := file.Write(data)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func ReadTopLevelIndexFromFile(file *os.File, offset uint64, size int) (*TopLevelIndex, error) {
	if _, err := file.Seek(int64(offset), 0); err != nil {
		return nil, err
	}
	data := make([]byte, size)
	if _, err := file.Read(data); err != nil {
		return nil, err
	}
	return DecodeTopLevelIndex(data)
}

type IndexEntry struct {
	Key    []byte
	Offset uint64
}

func (entry *IndexEntry) EncodedSize() int {
	keyLen := uint64(len(entry.Key))
	buf := make([]byte, binary.MaxVarintLen64)
	varintLen := binary.PutUvarint(buf, keyLen)
	return varintLen + len(entry.Key) + 8
}

func (entry *IndexEntry) EncodeTo(buf []byte) int {
	keyLen := uint64(len(entry.Key))
	pos := 0
	n := binary.PutUvarint(buf, keyLen)
	pos += n
	copy(buf[pos:], entry.Key[:keyLen])
	pos += len(entry.Key)
	binary.LittleEndian.PutUint64(buf[pos:], entry.Offset)
	pos += 8
	return pos
}

func (entry *IndexEntry) EncodeIndexEntry() []byte {
	size := entry.EncodedSize()
	buf := make([]byte, size)
	entry.EncodeTo(buf)
	return buf
}

func DecodeIndexEntry(buf []byte) (*IndexEntry, int, error) {
	pos := 0
	keyLen, n := binary.Uvarint(buf[pos:])
	if n <= 0 {
		return nil, 0, errors.New("invalid index key")
	}
	pos += n
	if pos+int(keyLen)+8 > len(buf) {
		return nil, 0, errors.New("buffer too small for key and offset")
	}
	key := make([]byte, int(keyLen))
	copy(key, buf[pos:pos+int(keyLen)])
	pos += int(keyLen)
	offset := binary.LittleEndian.Uint64(buf[pos:])
	pos += 8
	return &IndexEntry{Key: key, Offset: offset}, pos, nil
}

type IndexBlock struct {
	Entries []IndexEntry
}

func NewIndexBlock(config *config.Config) *IndexBlock {
	return &IndexBlock{
		Entries: make([]IndexEntry, 0, config.SSTable.IndexSegment.IndexBlockSize),
	}
}

func (ib *IndexBlock) AddEntry(entry IndexEntry) {
	ib.Entries = append(ib.Entries, entry)
}

func (ib *IndexBlock) Size() int {
	size := 4
	for i := range ib.Entries {
		size += ib.Entries[i].EncodedSize()
	}
	size += 4
	return size
}

func (ib *IndexBlock) Encode() []byte {
	totalSize := ib.Size()
	buf := make([]byte, totalSize)
	pos := 0
	binary.LittleEndian.PutUint32(buf[pos:], uint32(len(ib.Entries)))
	pos += 4
	for i := range ib.Entries {
		n := ib.Entries[i].EncodeTo(buf[pos:])
		pos += n
	}
	crc := crc32.ChecksumIEEE(buf[:pos])
	binary.LittleEndian.PutUint32(buf[pos:], crc)
	return buf
}

func DecodeIndexBlock(buf []byte) (*IndexBlock, error) {
	if len(buf) < 8 {
		return nil, errors.New("buffer too small for index block")
	}
	crcPos := len(buf) - 4
	expectedCRC := binary.LittleEndian.Uint32(buf[crcPos:])
	actualCRC := crc32.ChecksumIEEE(buf[:crcPos])
	if expectedCRC != actualCRC {
		return nil, errors.New("index block CRC mismatch")
	}
	numEntries := binary.LittleEndian.Uint32(buf[0:4])
	if numEntries == 0 {
		return &IndexBlock{Entries: []IndexEntry{}}, nil
	}
	entries := make([]IndexEntry, 0, numEntries)
	pos := 4
	for i := 0; i < int(numEntries); i++ {
		entry, n, err := DecodeIndexEntry(buf[pos:crcPos])
		if err != nil {
			return nil, fmt.Errorf("failed to decode index entry %d: %w", i, err)
		}
		entries = append(entries, *entry)
		pos += n
	}
	return &IndexBlock{
		Entries: entries,
	}, nil
}

func (ib *IndexBlock) FindBlock(key []byte) int {
	if len(ib.Entries) == 0 {
		return -1
	}
	if bytes.Compare(key, ib.Entries[0].Key) < 0 {
		return -1
	}
	lastIdx := len(ib.Entries) - 1
	if bytes.Compare(key, ib.Entries[lastIdx].Key) >= 0 {
		return lastIdx
	}
	left := 0
	right := lastIdx
	result := -1
	for left <= right {
		mid := left + (right-left)/2
		cmp := bytes.Compare(key, ib.Entries[mid].Key)
		if cmp < 0 {
			right = mid - 1
		} else if cmp == 0 {
			return mid
		} else {
			result = mid
			left = mid + 1
		}
	}
	return result
}

func (ib *IndexBlock) WriteToFile(file *os.File) (int, error) {
	data := ib.Encode()
	n, err := file.Write(data)
	if err != nil {
		return 0, fmt.Errorf("failed to write index block: %w", err)
	}
	return n, nil
}

func ReadIndexBlockFromFile(file *os.File, offset uint64, size uint32) (*IndexBlock, error) {
	if _, err := file.Seek(int64(offset), io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek failed: %w", err)
	}
	data := make([]byte, size)
	if _, err := io.ReadFull(file, data); err != nil {
		return nil, fmt.Errorf("read failed: %w", err)
	}
	return DecodeIndexBlock(data)
}

type IndexBlockCache struct {
	blocks    map[string]*IndexBlock
	keys      []string
	maxBlocks int
	hits      int64
	misses    int64
}

func NewIndexBlockCache(maxBlocks int) *IndexBlockCache {
	return &IndexBlockCache{
		blocks:    make(map[string]*IndexBlock),
		keys:      make([]string, 0, maxBlocks),
		maxBlocks: maxBlocks,
	}
}

func (c *IndexBlockCache) Get(key string) (*IndexBlock, bool) {
	block, ok := c.blocks[key]
	if ok {
		c.hits++
		c.moveToFront(key)
	} else {
		c.misses++
	}
	return block, ok
}

func (c *IndexBlockCache) Put(key string, block *IndexBlock) {
	if _, exists := c.blocks[key]; exists {
		c.moveToFront(key)
		c.blocks[key] = block
		return
	}
	if len(c.blocks) >= c.maxBlocks {
		c.evictLRU()
	}
	c.blocks[key] = block
	c.keys = append([]string{key}, c.keys...)
}

func (c *IndexBlockCache) moveToFront(key string) {
	for i, k := range c.keys {
		if k == key {
			c.keys = append(c.keys[:i], c.keys[i+1:]...)
			break
		}
	}
	c.keys = append([]string{key}, c.keys...)
}

func (c *IndexBlockCache) evictLRU() {
	if len(c.keys) == 0 {
		return
	}
	lru := c.keys[len(c.keys)-1]
	c.keys = c.keys[:len(c.keys)-1]
	delete(c.blocks, lru)
}

func (c *IndexBlockCache) Stats() (hits, misses int64, hitRate float64) {
	total := c.hits + c.misses
	if total == 0 {
		return c.hits, c.misses, 0.0
	}
	return c.hits, c.misses, float64(c.hits) / float64(total)
}

func (c *IndexBlockCache) Clear() {
	c.blocks = make(map[string]*IndexBlock)
	c.keys = make([]string, 0, c.maxBlocks)
	c.hits = 0
	c.misses = 0
}

type TwoLevelIndexBuilder struct {
	topLevel       *TopLevelIndex
	currentIndex   *IndexBlock
	indexBlocks    []*IndexBlock
	blocksPerIndex int
	indexOffset    uint64
}

func NewTwoLevelIndexBuilder(blocksPerIndex int, cnfig *config.Config) *TwoLevelIndexBuilder {
	if blocksPerIndex <= 0 {
		blocksPerIndex = cnfig.SSTable.IndexSegment.IndexBlockSize
	}
	return &TwoLevelIndexBuilder{
		topLevel:       NewTopLevelIndex(config.NewDefaultConfig()),
		currentIndex:   NewIndexBlock(config.NewDefaultConfig()),
		indexBlocks:    make([]*IndexBlock, 0),
		blocksPerIndex: blocksPerIndex,
	}
}

func (b *TwoLevelIndexBuilder) AddDataBlock(firstKey []byte, offset uint64) {
	b.currentIndex.AddEntry(IndexEntry{
		Key:    firstKey,
		Offset: offset,
	})
	if len(b.currentIndex.Entries) >= b.blocksPerIndex {
		b.finishIndexBlock()
	}
}

func (b *TwoLevelIndexBuilder) finishIndexBlock() {
	if len(b.currentIndex.Entries) == 0 {
		return
	}
	encoded := b.currentIndex.Encode()
	size := uint32(len(encoded))
	firstKey := b.currentIndex.Entries[0].Key
	b.topLevel.AddEntry(TopLevelIndexEntry{
		FirstKey: firstKey,
		Offset:   b.indexOffset,
		Size:     size,
	})
	b.indexBlocks = append(b.indexBlocks, b.currentIndex)
	b.indexOffset += uint64(size)
	b.currentIndex = NewIndexBlock(config.NewDefaultConfig())
}

func (b *TwoLevelIndexBuilder) Build() (*TopLevelIndex, []*IndexBlock) {
	b.finishIndexBlock()
	return b.topLevel, b.indexBlocks
}
