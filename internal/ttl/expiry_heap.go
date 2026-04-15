package ttl

import (
	"github.com/ajromen/LSM-KV-Engine/internal/shared"
)

type ExpiryHeap struct {
	entries []shared.TTLEntry
}

func NewExpiryHeap() *ExpiryHeap {
	return &ExpiryHeap{}
}

func (h *ExpiryHeap) Len() int { return len(h.entries) }

func (h *ExpiryHeap) Top() *shared.TTLEntry {
	if len(h.entries) == 0 {
		return nil
	}
	return &h.entries[0]
}

func (h *ExpiryHeap) Push(entry shared.TTLEntry) {
	h.entries = append(h.entries, entry)
	h.upheap(len(h.entries) - 1)
}

func (h *ExpiryHeap) Pop() *shared.TTLEntry {
	if len(h.entries) == 0 {
		return nil
	}
	top := h.entries[0]
	n := len(h.entries) - 1
	h.entries[0] = h.entries[n]
	h.entries = h.entries[:n]
	if len(h.entries) > 0 {
		h.downheap(0)
	}
	return &top
}

func (h *ExpiryHeap) upheap(i int) {
	for i > 0 {
		parent := (i - 1) / 2
		if h.entries[parent].ExpiresAt <= h.entries[i].ExpiresAt {
			break
		}
		h.entries[parent], h.entries[i] = h.entries[i], h.entries[parent]
		i = parent
	}
}

func (h *ExpiryHeap) downheap(i int) {
	n := len(h.entries)
	for {
		smallest := i
		left := 2*i + 1
		right := 2*i + 2

		if left < n && h.entries[left].ExpiresAt < h.entries[smallest].ExpiresAt {
			smallest = left
		}
		if right < n && h.entries[right].ExpiresAt < h.entries[smallest].ExpiresAt {
			smallest = right
		}
		if smallest == i {
			break
		}
		h.entries[i], h.entries[smallest] = h.entries[smallest], h.entries[i]
		i = smallest
	}
}
