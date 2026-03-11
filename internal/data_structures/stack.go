package data_structures

type Stack[T any] struct {
	data []T
}

func New[T any]() *Stack[T] {
	return &Stack[T]{
		data: make([]T, 0),
	}
}

func (s *Stack[T]) Push(value T) {
	s.data = append(s.data, value)
}

func (s *Stack[T]) Pop() T {
	if len(s.data) == 0 {
		var zero T
		return zero
	}
	var elem T = s.data[len(s.data)-1]
	s.data = s.data[:len(s.data)-1]
	return elem
}

func (s *Stack[T]) Peek() T {
	if len(s.data) == 0 {
		var zero T
		return zero
	}
	return s.data[len(s.data)-1]
}

func (s *Stack[T]) PeekBottom() T {
	if len(s.data) == 0 {
		var zero T
		return zero
	}
	return s.data[0]
}

func (s *Stack[T]) Len() int {
	return len(s.data)
}

func (s Stack[T]) Clone() Stack[T] {
	newStack := Stack[T]{}
	newStack.data = append(newStack.data, s.data...)
	return newStack
}

func (s Stack[T]) Data() []T {
	return s.data
}

func (s *Stack[T]) Clear() {
	s.data = s.data[:0]
}

func (s *Stack[T]) Empty() bool {
	return len(s.data) == 0
}
