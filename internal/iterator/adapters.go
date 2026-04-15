package iterator

type AdaptedIterator[T any] struct {
	inner    TypedSeekIterator[T]
	toEntry  func(T) Entry
	makeSeek func(key []byte) T
}

type TypedSeekIterator[T any] interface {
	Valid() bool
	SeekToFirst()
	SeekToLast()
	Seek(T)
	Next()
	Key() T
}

func NewAdaptedIterator[T any](
	inner TypedSeekIterator[T],
	toEntry func(T) Entry,
	makeSeek func(key []byte) T,
) Iterator[Entry] {
	return &AdaptedIterator[T]{
		inner:    inner,
		toEntry:  toEntry,
		makeSeek: makeSeek,
	}
}

func (a *AdaptedIterator[T]) Valid() bool {
	return a.inner != nil && a.inner.Valid()
}

func (a *AdaptedIterator[T]) SeekToFirst() {
	a.inner.SeekToFirst()
}

func (a *AdaptedIterator[T]) SeekToLast() {
	a.inner.SeekToLast()
}

func (a *AdaptedIterator[T]) Seek(key Entry) {
	target := a.makeSeek(key.Key)
	a.inner.Seek(target)
}

func (a *AdaptedIterator[T]) Next() {
	a.inner.Next()
}

func (a *AdaptedIterator[T]) Prev() {
	panic("AdaptedIterator: Prev not implemented")
}

func (a *AdaptedIterator[T]) Key() Entry {
	return a.Current()
}

func (a *AdaptedIterator[T]) Value() Entry {
	return a.Current()
}

func (a *AdaptedIterator[T]) Current() Entry {
	if !a.inner.Valid() {
		return Entry{}
	}
	return a.toEntry(a.inner.Key())
}
