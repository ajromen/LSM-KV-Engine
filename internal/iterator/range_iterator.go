package iterator

import "bytes"

type RangeIterator struct {
	db    *DBIterator
	lower []byte
	upper []byte
}

func NewRangeIterator(db *DBIterator, lower, upper []byte) *RangeIterator {
	it := &RangeIterator{
		db:    db,
		lower: append([]byte(nil), lower...),
		upper: append([]byte(nil), upper...),
	}
	it.SeekToFirst()
	return it
}

func (it *RangeIterator) Valid() bool {
	if it == nil || it.db == nil || !it.db.Valid() {
		return false
	}

	key := it.db.Current().Key
	if len(it.lower) > 0 && bytes.Compare(key, it.lower) < 0 {
		return false
	}
	if len(it.upper) > 0 && bytes.Compare(key, it.upper) > 0 {
		return false
	}
	return true
}

func (it *RangeIterator) SeekToFirst() {
	if it.db == nil {
		return
	}
	if len(it.lower) == 0 {
		it.db.SeekToFirst()
		return
	}
	it.db.Seek(Entry{Key: append([]byte(nil), it.lower...)})
}

func (it *RangeIterator) SeekToLast() {
	panic("RangeIterator: SeekToLast not implemented")
}

func (it *RangeIterator) Seek(key Entry) {
	if it.db == nil {
		return
	}

	target := copyEntry(key)
	if len(it.lower) > 0 && (len(target.Key) == 0 || bytes.Compare(target.Key, it.lower) < 0) {
		target.Key = append([]byte(nil), it.lower...)
	}

	it.db.Seek(target)
}

func (it *RangeIterator) Next() {
	if it.db == nil {
		return
	}
	it.db.Next()
}

func (it *RangeIterator) Prev() {
	panic("RangeIterator: Prev not implemented")
}

func (it *RangeIterator) Key() Entry {
	return it.Current()
}

func (it *RangeIterator) Value() Entry {
	return it.Current()
}

func (it *RangeIterator) Current() Entry {
	if it.db == nil {
		return Entry{}
	}
	return it.db.Current()
}
