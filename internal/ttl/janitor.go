package ttl

import (
	"sync"

	"github.com/ajromen/LSM-KV-Engine/internal/shared"
)

type Janitor struct {
	delFn func([]byte)
	heap  *ExpiryHeap
	index map[string]int64 // dva puta se drze svi ttl ako neko zna bolje nek uradi

	mu sync.Mutex
}

func NewTTLJanitor(delFn func([]byte)) *Janitor {
	j := &Janitor{
		delFn: delFn,
	}
	return j
}

func (j *Janitor) Init(heap *ExpiryHeap, index map[string]int64) {
	j.heap = heap
	j.index = index
}

func (j *Janitor) Run() {
	// run clock
	// check top

}

func (j *Janitor) AddTTL(entry shared.TTLEntry) {
	j.mu.Lock()
	j.heap.Push(entry)
	j.index[string(entry.Key)] = entry.ExpiresAt
	j.mu.Unlock()
}

func (j *Janitor) GetTTL(key string) (int64, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	expiresAt, ok := j.index[key]
	return expiresAt, ok
}

func (j *Janitor) ClearAll() {
	// TODO clear janitor
	j.mu.Lock()
	j.heap=NewExpiryHeap()
	j.index=make(map[string]int64)
	j.mu.Unlock()
}
