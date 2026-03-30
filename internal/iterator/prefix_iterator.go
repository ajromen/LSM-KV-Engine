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
