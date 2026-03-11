package include

// every type of iterator needs to implement this interface

type Iterator[T any] interface {
	Valid() bool
	SeekToFirst()
	SeekToLast()
	Seek(key T)
	Next()
	Prev()
	Key() T
	Value() T
}

func Compare[T any](a, b Iterator[T], cmp func(a, b T) int) Iterator[T] {
	if a == nil || !a.Valid() {
		return b
	}
	if b == nil || !b.Valid() {
		return a
	}
	if cmp(a.Key(), b.Key()) <= 0 {
		return a
	}
	return b
}
