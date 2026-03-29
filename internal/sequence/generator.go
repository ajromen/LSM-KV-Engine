package sequence

import "sync/atomic"

type SequenceGenerator struct {
	current atomic.Uint64
}

func NewSequenceGenerator(initial uint64) *SequenceGenerator {
	sg := &SequenceGenerator{}
	sg.current.Store(initial)
	return sg
}

func (sg *SequenceGenerator) Next() uint64 {
	return sg.current.Add(1)
}

func (sg *SequenceGenerator) Current() uint64 {
	return sg.current.Load()
}
