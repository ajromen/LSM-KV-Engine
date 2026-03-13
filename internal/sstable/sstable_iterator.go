package sstable

import (
	"bytes"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/data_structures"
	"github.com/ajromen/LSM-KV-Engine/internal/iterator"
)

type sstableBlockSource struct {
	reader    *SSTableReader
	numBlocks int
	blockSize int
}

func newSSTableBlockSource(reader *SSTableReader) *sstableBlockSource {
	return &sstableBlockSource{
		reader:    reader,
		numBlocks: int(reader.footer.NumDataBlocks),
		blockSize: reader.blockManager.BlockSize(),
	}
}

func (s *sstableBlockSource) loadBlock(n int) ([]byte, error) {
	key := block.BlockKey{
		FilePath: s.reader.filePath,
		Offset:   uint32(n),
	}
	return s.reader.blockManager.Read(key)
}

func (s *sstableBlockSource) blockIteratorRaw(n int) (*DataBlockIteratorRaw, error) {
	data, err := s.loadBlock(n)
	if err != nil {
		return nil, err
	}
	return NewDataBlockIteratorRaw(data, int(s.reader.footer.RestartInterval), s.reader.footer.EncodingType)
}

type SSTableIteratorRaw struct {
	src       *sstableBlockSource
	blockIdx  int
	blockIter *DataBlockIteratorRaw
	current   *Record
	valid     bool
}

func NewSSTableIteratorRaw(reader *SSTableReader) (*SSTableIteratorRaw, error) {
	it := &SSTableIteratorRaw{src: newSSTableBlockSource(reader)}
	it.SeekToFirst()
	return it, nil
}

func (it *SSTableIteratorRaw) Valid() bool { return it.valid }

func (it *SSTableIteratorRaw) SeekToFirst() {
	it.blockIdx = 0
	it.blockIter = nil
	it.valid = false
	it.advanceBlock()
}

func (it *SSTableIteratorRaw) SeekToLast() {
	for n := it.src.numBlocks - 1; n >= 0; n-- {
		bi, err := it.src.blockIteratorRaw(n)
		if err != nil {
			continue
		}
		bi.SeekToLast()
		if bi.Valid() {
			it.blockIdx = n
			it.blockIter = bi
			rec := bi.Key()
			it.current = &rec
			it.valid = true
			return
		}
	}
	it.valid = false
}

func (it *SSTableIteratorRaw) Seek(target Record) {
	it.SeekToFirst()
	for it.valid {
		if bytes.Compare(it.current.Key, target.Key) >= 0 {
			return
		}
		it.Next()
	}
}

func (it *SSTableIteratorRaw) Next() {
	if !it.valid {
		return
	}
	it.blockIter.Next()
	if it.blockIter.Valid() {
		rec := it.blockIter.Key()
		it.current = &rec
		return
	}
	it.blockIdx++
	it.advanceBlock()
}

func (it *SSTableIteratorRaw) Prev() {
	panic("SSTableIteratorRaw: Prev not implemented")
}

func (it *SSTableIteratorRaw) advanceBlock() {
	for it.blockIdx < it.src.numBlocks {
		bi, err := it.src.blockIteratorRaw(it.blockIdx)
		if err != nil {
			it.blockIdx++
			continue
		}
		bi.SeekToFirst()
		if bi.Valid() {
			it.blockIter = bi
			rec := bi.Key()
			it.current = &rec
			it.valid = true
			return
		}
		it.blockIdx++
	}
	it.current = nil
	it.valid = false
}

func (it *SSTableIteratorRaw) Key() Record   { return *it.current }
func (it *SSTableIteratorRaw) Value() Record { return *it.current }

type SSTableIterator struct {
	raw     *SSTableIteratorRaw
	current *Record
	valid   bool
}

func NewSSTableIterator(reader *SSTableReader) (*SSTableIterator, error) {
	raw, err := NewSSTableIteratorRaw(reader)
	if err != nil {
		return nil, err
	}
	it := &SSTableIterator{raw: raw}
	it.SeekToFirst()
	return it, nil
}

func (it *SSTableIterator) Valid() bool {
	return it.valid
}

func (it *SSTableIterator) SeekToFirst() {
	it.raw.SeekToFirst()
	it.advance(nil)
}

func (it *SSTableIterator) SeekToLast() {
	it.raw.SeekToFirst()
	var last *Record
	var prevKey []byte
	for it.raw.Valid() {
		rec := it.raw.Key()
		if !rec.Tombstone && !bytes.Equal(rec.Key, prevKey) {
			cp := rec
			last = &cp
			prevKey = rec.Key
		}
		it.raw.Next()
	}
	if last == nil {
		it.valid = false
		return
	}
	it.current = last
	it.valid = true
}

func (it *SSTableIterator) Seek(target Record) {
	it.raw.Seek(target)
	it.advance(nil)
}

func (it *SSTableIterator) Next() {
	if !it.valid {
		return
	}
	prevKey := it.current.Key
	it.raw.Next()
	it.advance(prevKey)
}

func (it *SSTableIterator) Prev() {
	panic("SSTableIterator: Prev not implemented")
}

func (it *SSTableIterator) advance(prevKey []byte) {
	for it.raw.Valid() {
		rec := it.raw.Key()
		if prevKey != nil && bytes.Equal(rec.Key, prevKey) {
			it.raw.Next()
			continue
		}
		if rec.Tombstone {
			prevKey = rec.Key
			it.raw.Next()
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

func (it *SSTableIterator) Key() Record   { return *it.current }
func (it *SSTableIterator) Value() Record { return *it.current }

type SSTableMergeIteratorRaw struct {
	structure data_structures.MergeStructure[Record]
	current   *Record
	valid     bool
}

var _ iterator.Iterator[Record] = (*SSTableMergeIteratorRaw)(nil)

func NewSSTableMergeIteratorRaw(readers []*SSTableReader, mergeStructure byte) (*SSTableMergeIteratorRaw, error) {
	wrapped := make([]iterator.Iterator[Record], 0, len(readers))
	for _, r := range readers {
		it, err := NewSSTableIteratorRaw(r)
		if err != nil {
			return nil, err
		}
		if it.Valid() {
			wrapped = append(wrapped, it)
		}
	}
	if len(wrapped) == 0 {
		return &SSTableMergeIteratorRaw{valid: false}, nil
	}
	structure := data_structures.NewMergeStructure(mergeStructure, wrapped, recordComparator)
	m := &SSTableMergeIteratorRaw{structure: structure}
	m.syncFromWinner()
	return m, nil
}

func (m *SSTableMergeIteratorRaw) Valid() bool   { return m.valid }
func (m *SSTableMergeIteratorRaw) Key() Record   { return *m.current }
func (m *SSTableMergeIteratorRaw) Value() Record { return *m.current }

func (m *SSTableMergeIteratorRaw) SeekToFirst() {
	for _, it := range m.structure.Iterators() {
		it.SeekToFirst()
		m.structure.Update(it)
	}
	m.syncFromWinner()
}

func (m *SSTableMergeIteratorRaw) SeekToLast() {
	for _, it := range m.structure.Iterators() {
		it.SeekToLast()
		m.structure.Update(it)
	}
	m.syncFromWinner()
}

func (m *SSTableMergeIteratorRaw) Seek(target Record) {
	for _, it := range m.structure.Iterators() {
		it.Seek(target)
		m.structure.Update(it)
	}
	m.syncFromWinner()
}

func (m *SSTableMergeIteratorRaw) Next() {
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

func (m *SSTableMergeIteratorRaw) Prev() {
	panic("SSTableMergeIteratorRaw: Prev not implemented")
}

func (m *SSTableMergeIteratorRaw) syncFromWinner() {
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

type SSTableMergeIterator struct {
	raw     *SSTableMergeIteratorRaw
	current *Record
	valid   bool
}

var _ iterator.Iterator[Record] = (*SSTableMergeIterator)(nil)

func NewSSTableMergeIterator(readers []*SSTableReader, mergeStructure byte) (*SSTableMergeIterator, error) {
	raw, err := NewSSTableMergeIteratorRaw(readers, mergeStructure)
	if err != nil {
		return nil, err
	}
	m := &SSTableMergeIterator{raw: raw}
	m.advance(nil)
	return m, nil
}

func (m *SSTableMergeIterator) Valid() bool   { return m.valid }
func (m *SSTableMergeIterator) Key() Record   { return *m.current }
func (m *SSTableMergeIterator) Value() Record { return *m.current }

func (m *SSTableMergeIterator) SeekToFirst() {
	m.raw.SeekToFirst()
	m.advance(nil)
}

func (m *SSTableMergeIterator) SeekToLast() {
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

func (m *SSTableMergeIterator) Seek(target Record) {
	m.raw.Seek(target)
	m.advance(nil)
}

func (m *SSTableMergeIterator) Next() {
	if !m.valid {
		return
	}
	prevKey := m.current.Key
	m.skipCurrentKey()
	m.advance(prevKey)
}

func (m *SSTableMergeIterator) Prev() {
	panic("SSTableMergeIterator: Prev not implemented")
}

func (m *SSTableMergeIterator) skipCurrentKey() {
	if !m.raw.Valid() {
		return
	}
	currentKey := append([]byte(nil), m.raw.Key().Key...)
	for m.raw.Valid() && bytes.Equal(m.raw.Key().Key, currentKey) {
		m.raw.Next()
	}
}

func (m *SSTableMergeIterator) advance(prevKey []byte) {
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
