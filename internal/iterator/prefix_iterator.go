package iterator

type PrefixIterator struct {
	rangeIterator *RangeIterator
}

func NewPrefixIterator(db *DBIterator, prefix []byte) *PrefixIterator {
	lower := append([]byte(nil), prefix...)
	upper := append(append([]byte(nil), prefix...), 0xFF)

	return &PrefixIterator{
		rangeIterator: NewRangeIterator(db, lower, upper),
	}
}

func (it *PrefixIterator) Valid() bool {
	return it != nil && it.rangeIterator != nil && it.rangeIterator.Valid()
}

func (it *PrefixIterator) SeekToFirst() {
	it.rangeIterator.SeekToFirst()
}

func (it *PrefixIterator) SeekToLast() {
	it.rangeIterator.SeekToLast()
}

func (it *PrefixIterator) Seek(key Entry) {
	it.rangeIterator.Seek(key)
}

func (it *PrefixIterator) Next() {
	it.rangeIterator.Next()
}

func (it *PrefixIterator) Prev() {
	it.rangeIterator.Prev()
}

func (it *PrefixIterator) Key() Entry {
	return it.Current()
}

func (it *PrefixIterator) Value() Entry {
	return it.Current()
}

func (it *PrefixIterator) Current() Entry {
	return it.rangeIterator.Current()
}
