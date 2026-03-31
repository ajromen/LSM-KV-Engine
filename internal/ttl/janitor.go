package ttl

import "github.com/ajromen/LSM-KV-Engine/internal/sstable"

type Janitor struct {
	delFn func([]byte)
	heap  *ExpiryHeap
}

func NewTTLJanitor(delFn func([]byte)) *Janitor {
	j := &Janitor{
		delFn: delFn,
		heap:  NewExpiryHeap(),
	}
	return j
}

func (j *Janitor) Init(entries []sstable.TTLEntry) {
	//create heap and start timer
}
