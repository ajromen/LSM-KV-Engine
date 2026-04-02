package notifier

import (
	"bytes"
	"sync"
)

type EventType byte

const (
	EventPut    EventType = 0
	EventDelete EventType = 1
)

func (e EventType) String() string {
	switch e {
	case EventPut:
		return "PUT"
	case EventDelete:
		return "DELETE"
	default:
		return "UNKNOWN"
	}
}

type Event struct {
	Type  EventType
	Key   []byte
	Value []byte
}

type Listener struct {
	id    uint64
	lower []byte
	upper []byte
	Ch    chan Event
}

type Notifier struct {
	mu        sync.RWMutex
	listeners map[uint64]*Listener
	nextID    uint64
}

func NewNotifier() *Notifier {
	return &Notifier{
		listeners: make(map[uint64]*Listener),
	}
}

func (n *Notifier) Subscribe(lower, upper []byte, bufferSize int) *Listener {
	n.mu.Lock()
	defer n.mu.Unlock()

	id := n.nextID
	n.nextID++

	l := &Listener{
		id:    id,
		lower: append([]byte(nil), lower...),
		upper: append([]byte(nil), upper...),
		Ch:    make(chan Event, bufferSize),
	}
	n.listeners[id] = l
	return l
}

func (n *Notifier) Unsubscribe(l *Listener) {
	if l == nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()

	if _, ok := n.listeners[l.id]; ok {
		delete(n.listeners, l.id)
		close(l.Ch)
	}
}

func (n *Notifier) NotifyPut(key, value []byte) {
	n.notify(Event{
		Type:  EventPut,
		Key:   append([]byte(nil), key...),
		Value: append([]byte(nil), value...),
	})
}

func (n *Notifier) NotifyDelete(key []byte) {
	n.notify(Event{
		Type:  EventDelete,
		Key:   append([]byte(nil), key...),
		Value: nil,
	})
}

func (n *Notifier) notify(e Event) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	for _, l := range n.listeners {
		if l.inRange(e.Key) {

			select {
			case l.Ch <- e:
			default:
			}
		}
	}
}

func (l *Listener) inRange(key []byte) bool {
	if len(l.lower) > 0 && bytes.Compare(key, l.lower) < 0 {
		return false
	}
	if len(l.upper) > 0 && bytes.Compare(key, l.upper) > 0 {
		return false
	}
	return true
}
