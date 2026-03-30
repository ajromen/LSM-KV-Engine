package iterator

import "bytes"

type Entry struct {
	Key        []byte
	Value      []byte
	Tombstone  bool
	SequenceID uint64
}

type DBIterator struct {
	memIterator   Iterator[Entry]
	tableIterator Iterator[Entry]
	current       *Entry
	valid         bool
}

const (
	dbWinnerNone = iota
	dbWinnerMem
	dbWinnerSST
)

func NewDBIterator(memIterator Iterator[Entry], tableIterator Iterator[Entry]) *DBIterator {
	it := &DBIterator{
		memIterator:   memIterator,
		tableIterator: tableIterator,
	}
	it.SeekToFirst()
	return it
}

func (it *DBIterator) Valid() bool {
	return it != nil && it.valid
}

func (it *DBIterator) SeekToFirst() {
	if it.memIterator != nil {
		it.memIterator.SeekToFirst()
	}
	if it.tableIterator != nil {
		it.tableIterator.SeekToFirst()
	}
	it.advance()
}

func (it *DBIterator) SeekToLast() {
	panic("DBIterator: SeekToLast not implemented")
}

func (it *DBIterator) Seek(key Entry) {
	if it.memIterator != nil {
		it.memIterator.Seek(key)
	}
	if it.tableIterator != nil {
		it.tableIterator.Seek(key)
	}
	it.advance()
}

func (it *DBIterator) Next() {
	if !it.valid {
		return
	}
	it.advance()
}

func (it *DBIterator) Prev() {
	panic("DBIterator: Prev not implemented")
}

func (it *DBIterator) Key() Entry {
	return it.Current()
}

func (it *DBIterator) Value() Entry {
	return it.Current()
}

func (it *DBIterator) Current() Entry {
	if !it.valid || it.current == nil {
		return Entry{}
	}
	return copyEntry(*it.current)
}

func (it *DBIterator) advance() {
	for {
		switch it.selectWinner() {
		case dbWinnerNone:
			it.current = nil
			it.valid = false
			return

		case dbWinnerMem:
			entry := it.memIterator.Key()
			keyCopy := append([]byte(nil), entry.Key...)
			valCopy := append([]byte(nil), entry.Value...)

			it.skipKey(keyCopy)

			if entry.Tombstone {
				continue
			}

			it.current = &Entry{
				Key:        keyCopy,
				Value:      valCopy,
				Tombstone:  false,
				SequenceID: entry.SequenceID,
			}
			it.valid = true
			return

		case dbWinnerSST:
			entry := it.tableIterator.Key()
			keyCopy := append([]byte(nil), entry.Key...)
			valCopy := append([]byte(nil), entry.Value...)

			it.skipKey(keyCopy)

			if entry.Tombstone {
				continue
			}

			it.current = &Entry{
				Key:        keyCopy,
				Value:      valCopy,
				Tombstone:  false,
				SequenceID: entry.SequenceID,
			}
			it.valid = true
			return
		}
	}
}

func (it *DBIterator) skipKey(key []byte) {
	for it.memIterator != nil && it.memIterator.Valid() && bytes.Equal(it.memIterator.Key().Key, key) {
		it.memIterator.Next()
	}
	for it.tableIterator != nil && it.tableIterator.Valid() && bytes.Equal(it.tableIterator.Key().Key, key) {
		it.tableIterator.Next()
	}
}

func (it *DBIterator) selectWinner() int {
	memValid := it.memIterator != nil && it.memIterator.Valid()
	sstValid := it.tableIterator != nil && it.tableIterator.Valid()

	if !memValid && !sstValid {
		return dbWinnerNone
	}
	if memValid && !sstValid {
		return dbWinnerMem
	}
	if !memValid && sstValid {
		return dbWinnerSST
	}

	memEntry := it.memIterator.Key()
	sstEntry := it.tableIterator.Key()

	if compareEntries(memEntry, sstEntry) <= 0 {
		return dbWinnerMem
	}
	return dbWinnerSST
}

func compareEntries(a, b Entry) int {
	if c := bytes.Compare(a.Key, b.Key); c != 0 {
		return c
	}

	// veći sequence_id = noviji → ide prvi
	if a.SequenceID > b.SequenceID {
		return -1
	}
	if a.SequenceID < b.SequenceID {
		return 1
	}
	return 0
}

func copyEntry(e Entry) Entry {
	return Entry{
		Key:        append([]byte(nil), e.Key...),
		Value:      append([]byte(nil), e.Value...),
		Tombstone:  e.Tombstone,
		SequenceID: e.SequenceID,
	}
}
