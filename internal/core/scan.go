package core

import (
	"fmt"

	"github.com/ajromen/LSM-KV-Engine/internal/iterator"
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
	for i := 0; i < skip && it.Valid(); i++ {
		it.Next()
	}
	results := make([]ScanResult, 0, pageSize)
	for it.Valid() && len(results) < pageSize {
		entry := it.Current()
		results = append(results, ScanResult{
			Key:   string(entry.Key),
			Value: string(entry.Value),
		})
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
	for i := 0; i < skip && it.Valid(); i++ {
		it.Next()
	}
	results := make([]ScanResult, 0, pageSize)
	for it.Valid() && len(results) < pageSize {
		entry := it.Current()
		results = append(results, ScanResult{
			Key:   string(entry.Key),
			Value: string(entry.Value),
		})
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
	if a.it == nil || !a.it.Valid() {
		return ScanResult{}, false
	}
	entry := a.it.Key()
	result := ScanResult{
		Key:   string(entry.Key),
		Value: string(entry.Value),
	}
	a.it.Next()
	return result, true
}

// Valid reports whether the iterator has more entries
func (a *ActiveIterator) Valid() bool {
	return a.it != nil && a.it.Valid()
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
