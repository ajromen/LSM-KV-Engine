package iterator

type Entry struct {
	Key           []byte
	Value         []byte
	Tombstone     bool
	TimestampHigh uint64
	TimestampLow  uint64
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
