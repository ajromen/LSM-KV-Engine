package core

import (
	"fmt"

	"github.com/ajromen/LSM-KV-Engine/internal/iterator"
	"github.com/ajromen/LSM-KV-Engine/internal/merge"
)

// ScanResult is a single key-value pair returned from scan operations
type ScanResult struct {
	Key   string
	Value string
}

// RangeScan returns a page of records whose keys fall within [lower, upper]
// pageNumber is 0-based, pageSize is the number of records per page
func (engine *Engine) RangeScan(lower, upper string, pageNumber, pageSize int) ([]ScanResult, error) {
	if pageSize <= 0 {
		return nil, fmt.Errorf("pageSize must be > 0")
	}

	it, err := engine.lsm.NewRangeIterator([]byte(lower), []byte(upper))
	if err != nil {
		return nil, err
	}

	skip := pageNumber * pageSize
	visibleSeen := 0

	for it.Valid() && visibleSeen < skip {
		entry := it.Current()
		if !merge.IsProbKey(entry.Key) {
			visibleSeen++
		}
		it.Next()
	}

	results := make([]ScanResult, 0, pageSize)
	for it.Valid() && len(results) < pageSize {
		entry := it.Current()
		if !merge.IsProbKey(entry.Key) {
			results = append(results, ScanResult{
				Key:   string(entry.Key),
				Value: string(entry.Value),
			})
		}
		it.Next()
	}

	return results, nil
}

// PrefixScan returns a page of records whose keys start with the given prefix
// pageNumber is 0-based, pageSize is the number of records per page
func (engine *Engine) PrefixScan(prefix string, pageNumber, pageSize int) ([]ScanResult, error) {
	if pageSize <= 0 {
		return nil, fmt.Errorf("pageSize must be > 0")
	}

	it, err := engine.lsm.NewPrefixIterator([]byte(prefix))
	if err != nil {
		return nil, err
	}

	skip := pageNumber * pageSize
	visibleSeen := 0

	for it.Valid() && visibleSeen < skip {
		entry := it.Current()
		if !merge.IsProbKey(entry.Key) {
			visibleSeen++
		}
		it.Next()
	}

	results := make([]ScanResult, 0, pageSize)
	for it.Valid() && len(results) < pageSize {
		entry := it.Current()
		if !merge.IsProbKey(entry.Key) {
			results = append(results, ScanResult{
				Key:   string(entry.Key),
				Value: string(entry.Value),
			})
		}
		it.Next()
	}

	return results, nil
}

// ActiveIterator holds an open interactive iterator session
type ActiveIterator struct {
	it iterator.Iterator[iterator.Entry]
}

// Next returns current entry and advances. Returns false if exhausted
func (a *ActiveIterator) Next() (ScanResult, bool) {
	for a.it != nil && a.it.Valid() {
		entry := a.it.Key()
		a.it.Next()

		if merge.IsProbKey(entry.Key) {
			continue
		}

		return ScanResult{
			Key:   string(entry.Key),
			Value: string(entry.Value),
		}, true
	}

	return ScanResult{}, false
}

// Valid reports whether the iterator has more entries
func (a *ActiveIterator) Valid() bool {
	if a.it == nil {
		return false
	}

	for a.it.Valid() {
		entry := a.it.Key()
		if !merge.IsProbKey(entry.Key) {
			return true
		}
		a.it.Next()
	}

	return false
}

// RangeIterate creates an interactive iterator over [lower, upper]
func (engine *Engine) RangeIterate(lower, upper string) (*ActiveIterator, error) {
	it, err := engine.lsm.NewRangeIterator([]byte(lower), []byte(upper))
	if err != nil {
		return nil, err
	}
	return &ActiveIterator{it: it}, nil
}

// PrefixIterate creates an interactive iterator over keys starting with prefix
func (engine *Engine) PrefixIterate(prefix string) (*ActiveIterator, error) {
	it, err := engine.lsm.NewPrefixIterator([]byte(prefix))
	if err != nil {
		return nil, err
	}
	return &ActiveIterator{it: it}, nil
}
