package ttl

import (
	"fmt"
	"sync"
	"time"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/shared"
)

type Janitor struct {
	delFn  func([]byte)
	heap   *ExpiryHeap
	index  map[string]int64 // dva puta se drze svi ttl ako neko zna bolje nek uradi
	stopCh chan struct{}
	mu     sync.Mutex
}

func NewTTLJanitor(delFn func([]byte)) *Janitor {
	j := &Janitor{
		delFn:  delFn,
		stopCh: make(chan struct{}),
	}
	return j
}

func (j *Janitor) Init(heap *ExpiryHeap, index map[string]int64) {
	j.heap = heap
	j.index = index
	if config.GetSettings().Debug {
		fmt.Printf("Janitor loaded %d ttls \n", len(j.index))
	}
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

func (j *Janitor) Run() {
	refreshRate := time.Duration(config.GetSettings().TTL.RefreshRate) * time.Millisecond
	for {
		select {
		case <-j.stopCh:
			return
		case <-time.After(refreshRate):
			j.evict()
		}
	}
}

func (j *Janitor) evict() {
	now := time.Now().UnixMilli()
	j.mu.Lock()
	var expired [][]byte
	for j.heap.Len() > 0 && j.heap.Top().ExpiresAt <= now {
		entry := j.heap.Pop()
		if entry == nil {
			break
		}
		if len(entry.Key) == 0 {
			continue
		}
		delete(j.index, string(entry.Key))
		expired = append(expired, entry.Key)
	}
	j.mu.Unlock()

	for _, key := range expired {
		j.delFn(key)
	}
}

func (j *Janitor) ClearAll() {
	// TODO clear janitor
	j.mu.Lock()
	j.heap = NewExpiryHeap()
	j.index = make(map[string]int64)
	j.mu.Unlock()
}

func (j *Janitor) Stop() {
	close(j.stopCh)
}
