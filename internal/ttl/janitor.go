package ttl

import (
	"fmt"
	"sync"
	"time"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/notifier"
	"github.com/ajromen/LSM-KV-Engine/internal/shared"
)

type Janitor struct {
	heap     *ExpiryHeap
	notifier *notifier.Notifier
	stopCh   chan struct{}
	mu       sync.Mutex
}

func NewTTLJanitor(notifier *notifier.Notifier) *Janitor {
	j := &Janitor{
		notifier: notifier,
		stopCh:   make(chan struct{}),
	}
	return j
}

func (j *Janitor) Init(heap *ExpiryHeap) {
	j.heap = heap
	if config.GetSettings().Debug {
		fmt.Printf("Janitor loaded %d ttls \n", j.heap.Len())
	}
}

func (j *Janitor) AddTTL(entry shared.TTLEntry) {
	j.mu.Lock()
	j.heap.Push(entry)
	j.mu.Unlock()
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
		expired = append(expired, entry.Key)
	}
	j.mu.Unlock()

	for _, key := range expired {
		j.notifier.NotifyDelete(key)
	}
}

func (j *Janitor) ClearAll() {
	j.mu.Lock()
	j.heap = NewExpiryHeap()
	j.mu.Unlock()
}

func (j *Janitor) Stop() {
	close(j.stopCh)
}
