package sstable

import (
	"bytes"
	"container/heap"
	"sort"

	"github.com/ajromen/LSM-KV-Engine/internal/shared"
	"github.com/ajromen/LSM-KV-Engine/internal/utils"
)

// FragmentedTombstoneIter iterates over pre-fragmented tombstones for one SSTable.
// Because fragments are non-overlapping and sorted by StartKey, EndKeys are also
// monotonically increasing, making Seek-by-EndKey a valid binary search.
type FragmentedTombstoneIter struct {
	fragments []shared.RangeDelEntry
	pos       int
}

func NewFragmentedTombstoneIter(fragments []shared.RangeDelEntry) *FragmentedTombstoneIter {
	return &FragmentedTombstoneIter{fragments: fragments}
}

func (it *FragmentedTombstoneIter) Valid() bool {
	return it.pos < len(it.fragments)
}

func (it *FragmentedTombstoneIter) StartKey() []byte {
	return it.fragments[it.pos].StartKey
}

func (it *FragmentedTombstoneIter) EndKey() []byte {
	return it.fragments[it.pos].EndKey
}

func (it *FragmentedTombstoneIter) SeqId() uint64 {
	return it.fragments[it.pos].SeqId
}

func (it *FragmentedTombstoneIter) Next() {
	it.pos++
}

func (it *FragmentedTombstoneIter) Prev() {}

func (it *FragmentedTombstoneIter) SeekToFirst() {
	it.pos = 0
}

func (it *FragmentedTombstoneIter) SeekToLast() {}

func (it *FragmentedTombstoneIter) Key() []byte { return nil }

func (it *FragmentedTombstoneIter) Value() []byte { return nil }

// Seek positions at first fragment whose EndKey > key.
func (it *FragmentedTombstoneIter) Seek(key []byte) {
	lo, hi := 0, len(it.fragments)
	for lo < hi {
		mid := lo + (hi-lo)/2
		if bytes.Compare(it.fragments[mid].EndKey, key) <= 0 {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	it.pos = lo
}

// --- inactive heap: min by StartKey ---
// Holds tombstones whose range has not yet begun (StartKey > scan position).

type inactiveHeap []*FragmentedTombstoneIter

func (h inactiveHeap) Len() int {
	return len(h)
}

func (h inactiveHeap) Less(i, j int) bool {
	return bytes.Compare(h[i].StartKey(), h[j].StartKey()) < 0
}

func (h inactiveHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *inactiveHeap) Push(x interface{}) {
	*h = append(*h, x.(*FragmentedTombstoneIter))
}

func (h *inactiveHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// activeEndHeap: min-heap by EndKey.
// Holds tombstones currently overlapping the scan position (StartKey <= pos < EndKey).
// The minimum EndKey tells us which tombstone expires next.
type activeEndHeap []*FragmentedTombstoneIter

func (h activeEndHeap) Len() int {
	return len(h)
}

func (h activeEndHeap) Less(i, j int) bool {
	return bytes.Compare(h[i].EndKey(), h[j].EndKey()) < 0
}

func (h activeEndHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *activeEndHeap) Push(x interface{}) {
	*h = append(*h, x.(*FragmentedTombstoneIter))
}

func (h *activeEndHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// activeSeqSet: ordered set of active tombstone iterators sorted by SeqId
// descending. The head is always the highest-seqId active tombstone, giving
// O(1) access to the maximum covering sequence number.

type activeSeqSet struct {
	iters []*FragmentedTombstoneIter
}

func (s *activeSeqSet) Insert(it *FragmentedTombstoneIter) {
	pos := sort.Search(len(s.iters), func(i int) bool {
		return s.iters[i].SeqId() <= it.SeqId()
	})
	s.iters = append(s.iters, nil)
	copy(s.iters[pos+1:], s.iters[pos:])
	s.iters[pos] = it
}

func (s *activeSeqSet) Remove(it *FragmentedTombstoneIter) {
	for i, iter := range s.iters {
		if iter == it {
			s.iters = append(s.iters[:i], s.iters[i+1:]...)
			return
		}
	}
}

func (s *activeSeqSet) Max() uint64 {
	if len(s.iters) == 0 {
		return 0
	}
	return s.iters[0].SeqId()
}

func (s *activeSeqSet) Len() int {
	return len(s.iters)
}

func (s *activeSeqSet) Clear() {
	s.iters = s.iters[:0]
}

// --- ForwardRangeDelChecker ---

// ForwardRangeDelChecker implements the three-structure approach from RocksDB's
// ForwardRangeDelIterator. Keys must be presented in ascending order.
//
//   inactive  — tombstones not yet reached, ordered by StartKey (min-heap)
//   active    — tombstones covering current position, ordered by EndKey (min-heap)
//   seqNums   — seqIds of active tombstones, ordered descending (head = max)
//
// ShouldDelete is O(1) amortised: each tombstone transitions state at most once
// per scan, paying O(log K) for heap ops where K = number of SSTable iterators.

type ForwardRangeDelChecker struct {
	inactive inactiveHeap
	active   activeEndHeap
	seqNums  activeSeqSet
	allIters []*FragmentedTombstoneIter
}

func NewForwardRangeDelChecker(readers []*SSTableReader) *ForwardRangeDelChecker {
	c := &ForwardRangeDelChecker{}
	for _, r := range readers {
		if err := r.LoadRangeDels(); err != nil || len(r.fragmentedRangeDels) == 0 {
			continue
		}
		c.allIters = append(c.allIters, NewFragmentedTombstoneIter(r.fragmentedRangeDels))
	}
	c.SeekToFirst()
	return c
}

func (c *ForwardRangeDelChecker) SeekToFirst() {
	c.inactive = c.inactive[:0]
	c.active = c.active[:0]
	c.seqNums.Clear()
	for _, it := range c.allIters {
		it.SeekToFirst()
		if it.Valid() {
			heap.Push(&c.inactive, it)
		}
	}
}

func (c *ForwardRangeDelChecker) Seek(key []byte) {
	c.inactive = c.inactive[:0]
	c.active = c.active[:0]
	c.seqNums.Clear()
	for _, it := range c.allIters {
		it.Seek(key)
		if !it.Valid() {
			continue
		}
		if bytes.Compare(it.StartKey(), key) <= 0 {
			heap.Push(&c.active, it)
			c.seqNums.Insert(it)
		} else {
			heap.Push(&c.inactive, it)
		}
	}
}

func (c *ForwardRangeDelChecker) ShouldDelete(key []byte, keySeqId uint64) bool {
	// retire active tombstones whose range ended at or before key
	for len(c.active) > 0 {
		top := c.active[0]
		if bytes.Compare(top.EndKey(), key) > 0 {
			break
		}
		heap.Pop(&c.active)
		c.seqNums.Remove(top)
		top.Next()
		c.classifyIter(top, key)
	}
	// activate tombstones whose range now includes key
	for len(c.inactive) > 0 {
		top := c.inactive[0]
		if bytes.Compare(top.StartKey(), key) > 0 {
			break
		}
		heap.Pop(&c.inactive)
		for top.Valid() && bytes.Compare(top.EndKey(), key) <= 0 {
			top.Next()
		}
		if top.Valid() {
			heap.Push(&c.active, top)
			c.seqNums.Insert(top)
		}
	}
	return c.seqNums.Max() > keySeqId
}

func (c *ForwardRangeDelChecker) classifyIter(it *FragmentedTombstoneIter, currentKey []byte) {
	if !it.Valid() {
		return
	}
	if bytes.Compare(it.StartKey(), currentKey) <= 0 {
		for it.Valid() && bytes.Compare(it.EndKey(), currentKey) <= 0 {
			it.Next()
		}
		if it.Valid() {
			heap.Push(&c.active, it)
			c.seqNums.Insert(it)
		}
	} else {
		heap.Push(&c.inactive, it)
	}
}

// MergeAndFragment collects range tombstones from multiple readers,
// re-fragments their union, returns a single sorted slice.
func MergeAndFragment(readers []*SSTableReader) []shared.RangeDelEntry {
	var all []shared.RangeDelEntry
	for _, r := range readers {
		if err := r.LoadRangeDels(); err == nil {
			all = append(all, r.fragmentedRangeDels...)
		}
	}
	return utils.FragmentRangeTombstones(all)
}
