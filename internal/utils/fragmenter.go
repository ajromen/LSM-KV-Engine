package utils

import (
	"bytes"
	"sort"

	"github.com/ajromen/LSM-KV-Engine/internal/shared"
)

// FragmentRangeTombstones converts a list of possibly-overlapping tombstones
// into an equivalent set of non-overlapping fragments sorted by StartKey
// The key insight : [a,z)@10 and [c,d)@4 cannot be binary-searched directly
// because neither sort order (by start nor by end) and neither lets you find
// all covering tombstones in O(logn) so we use fragmentation
// After fragmentation result is: [a,c)@10 [c,d)@10 [c,d)@4 [d,z)@10
// which is non-overlapping and binary-searchable
func FragmentRangeTombstones(tombstones []shared.RangeDelEntry) []shared.RangeDelEntry {
	if len(tombstones) == 0 {
		return nil
	}
	// collect every unique bounday key
	seen := make(map[string]struct{}) // represents set
	var boundaries [][]byte
	for _, t := range tombstones {
		for _, k := range [][]byte{t.StartKey, t.EndKey} {
			if _, ok := seen[string(k)]; !ok {
				seen[string(k)] = struct{}{}
				cp := make([]byte, len(k))
				copy(cp, k)
				boundaries = append(boundaries, cp)
			}
		}
	}
	sort.Slice(boundaries, func(i, j int) bool {
		return bytes.Compare(boundaries[i], boundaries[j]) < 0
	})
	// for each adjacent pair, find the max-seqId -> tombstone whose range fully contains this sub-interval
	var fragments []shared.RangeDelEntry
	for i := 0; i+1 < len(boundaries); i++ {
		start, end := boundaries[i], boundaries[i+1]
		var maxSeq uint64
		for _, t := range tombstones {
			if bytes.Compare(t.StartKey, start) <= 0 && bytes.Compare(end, t.EndKey) <= 0 && t.SeqId > maxSeq {
				maxSeq = t.SeqId
			}
		}
		if maxSeq > 0 {
			fragments = append(fragments, shared.RangeDelEntry{
				StartKey: start,
				EndKey:   end,
				SeqId:    maxSeq,
			})
		}
	}
	return fragments
}

func IsFragmented(entries []shared.RangeDelEntry) bool {
	if len(entries) == 0 {
		return true
	}
	for i := 1; i < len(entries); i++ {
		prev := entries[i-1]
		curr := entries[i]
		if bytes.Compare(prev.EndKey, curr.StartKey) <= 0 {
			continue
		}
		if bytes.Equal(prev.StartKey, curr.StartKey) &&
			bytes.Equal(prev.EndKey, curr.EndKey) {
			continue
		}
		return false
	}

	return true
}
