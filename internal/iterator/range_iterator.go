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
