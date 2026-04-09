package core

import (
	"sync"

	"github.com/ajromen/LSM-KV-Engine/internal/notifier"
)

const scanBufferSize = 32

// PagedRangeScan holds state for interactive paginated range scan
// The listener is scoped to the key range of the current page only
type PagedRangeScan struct {
	engine   *Engine
	lower    string
	upper    string
	pageSize int

	mu          sync.Mutex
	currentPage []ScanResult
	pageNumber  int
	dirty       bool
	listener    *notifier.Listener
	stopCh      chan struct{}
}

// NewPagedRangeScan creates a stateful paginated range scan over [lower, upper]
func (engine *Engine) NewPagedRangeScan(lower, upper string, pageSize int) (*PagedRangeScan, error) {
	s := &PagedRangeScan{
		engine:   engine,
		lower:    lower,
		upper:    upper,
		pageSize: pageSize,
		stopCh:   make(chan struct{}),
	}
	if err := s.fetchCurrentPage(); err != nil {
		return nil, err
	}
	s.resubscribe()
	go s.watchEvents()
	return s, nil
}

func (s *PagedRangeScan) resubscribe() {
	if s.listener != nil {
		s.engine.notifier.Unsubscribe(s.listener)
	}
	var lo, hi string
	if len(s.currentPage) > 0 {
		lo = s.currentPage[0].Key
		hi = s.currentPage[len(s.currentPage)-1].Key
	} else {
		lo = s.lower
		hi = s.upper
	}
	s.listener = s.engine.notifier.Subscribe([]byte(lo), []byte(hi), scanBufferSize)
}

func (s *PagedRangeScan) watchEvents() {
	for {
		select {
		case _, ok := <-s.listener.Ch:
			if !ok {
				return
			}
			s.mu.Lock()
			s.dirty = true
			s.mu.Unlock()
		case <-s.stopCh:
			return
		}
	}
}

func (s *PagedRangeScan) fetchCurrentPage() error {
	results, err := s.engine.RangeScan(s.lower, s.upper, s.pageNumber, s.pageSize)
	if err != nil {
		return err
	}
	s.currentPage = results
	return nil
}

func (s *PagedRangeScan) CurrentPage() ([]ScanResult, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	wasRefreshed := false
	if s.dirty {
		s.fetchCurrentPage()
		s.resubscribe()
		s.dirty = false
		wasRefreshed = true
	}
	return s.currentPage, wasRefreshed
}

func (s *PagedRangeScan) NextPage() ([]ScanResult, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pageNumber++
	if err := s.fetchCurrentPage(); err != nil || len(s.currentPage) == 0 {
		s.pageNumber--
		s.fetchCurrentPage()
		s.resubscribe()
		s.dirty = false
		return nil, false
	}
	s.resubscribe()
	s.dirty = false
	return s.currentPage, true
}

func (s *PagedRangeScan) PrevPage() []ScanResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pageNumber > 0 {
		s.pageNumber--
		s.fetchCurrentPage()
		s.resubscribe()
		s.dirty = false
	}
	return s.currentPage
}

func (s *PagedRangeScan) PageNumber() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pageNumber
}

func (s *PagedRangeScan) IsDirty() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dirty
}

func (s *PagedRangeScan) Close() {
	close(s.stopCh)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		s.engine.notifier.Unsubscribe(s.listener)
		s.listener = nil
	}
}

// PagedPrefixScan holds state for interactive paginated prefix scan
type PagedPrefixScan struct {
	engine   *Engine
	prefix   string
	pageSize int

	mu          sync.Mutex
	currentPage []ScanResult
	pageNumber  int
	dirty       bool
	listener    *notifier.Listener
	stopCh      chan struct{}
}

// NewPagedPrefixScan creates a stateful paginated prefix scan
func (engine *Engine) NewPagedPrefixScan(prefix string, pageSize int) (*PagedPrefixScan, error) {
	s := &PagedPrefixScan{
		engine:   engine,
		prefix:   prefix,
		pageSize: pageSize,
		stopCh:   make(chan struct{}),
	}
	if err := s.fetchCurrentPage(); err != nil {
		return nil, err
	}
	s.resubscribe()
	go s.watchEvents()
	return s, nil
}

func (s *PagedPrefixScan) resubscribe() {
	if s.listener != nil {
		s.engine.notifier.Unsubscribe(s.listener)
	}
	var lo, hi string
	if len(s.currentPage) > 0 {
		lo = s.currentPage[0].Key
		hi = s.currentPage[len(s.currentPage)-1].Key
	} else {
		lo = s.prefix
		hi = s.prefix
	}
	s.listener = s.engine.notifier.Subscribe([]byte(lo), []byte(hi), scanBufferSize)
}

func (s *PagedPrefixScan) watchEvents() {
	for {
		select {
		case _, ok := <-s.listener.Ch:
			if !ok {
				return
			}
			s.mu.Lock()
			s.dirty = true
			s.mu.Unlock()
		case <-s.stopCh:
			return
		}
	}
}

func (s *PagedPrefixScan) fetchCurrentPage() error {
	results, err := s.engine.PrefixScan(s.prefix, s.pageNumber, s.pageSize)
	if err != nil {
		return err
	}
	s.currentPage = results
	return nil
}

func (s *PagedPrefixScan) CurrentPage() ([]ScanResult, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	wasRefreshed := false
	if s.dirty {
		s.fetchCurrentPage()
		s.resubscribe()
		s.dirty = false
		wasRefreshed = true
	}
	return s.currentPage, wasRefreshed
}

func (s *PagedPrefixScan) NextPage() ([]ScanResult, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pageNumber++
	if err := s.fetchCurrentPage(); err != nil || len(s.currentPage) == 0 {
		s.pageNumber--
		s.fetchCurrentPage()
		s.resubscribe()
		s.dirty = false
		return nil, false
	}
	s.resubscribe()
	s.dirty = false
	return s.currentPage, true
}

func (s *PagedPrefixScan) PrevPage() []ScanResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pageNumber > 0 {
		s.pageNumber--
		s.fetchCurrentPage()
		s.resubscribe()
		s.dirty = false
	}
	return s.currentPage
}

func (s *PagedPrefixScan) PageNumber() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pageNumber
}

func (s *PagedPrefixScan) IsDirty() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dirty
}

func (s *PagedPrefixScan) Close() {
	close(s.stopCh)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		s.engine.notifier.Unsubscribe(s.listener)
		s.listener = nil
	}
}
