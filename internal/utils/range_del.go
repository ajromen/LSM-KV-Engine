package utils

import (
	"bytes"

	"github.com/ajromen/LSM-KV-Engine/internal/shared"
)

// IsCoveredByRangeTombstone binary searches sorted non-overlapping fragments.
// A fragment covers key when: StartKey <= key < EndKey AND fragment.SeqId > keySeqId.
func IsCoveredByRangeTombstone(fragments []shared.RangeDelEntry, key []byte, keySeqId uint64) bool {
	n := len(fragments)
	if n == 0 {
		return false
	}
	lo, hi, pos := 0, n-1, -1
	for lo <= hi {
		mid := lo + (hi-lo)/2
		if bytes.Compare(fragments[mid].StartKey, key) <= 0 {
			pos = mid
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	if pos < 0 {
		return false
	}
	f := fragments[pos]
	return bytes.Compare(key, f.EndKey) < 0 && f.SeqId > keySeqId
}
