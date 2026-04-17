package utils

import (
	"fmt"
	"testing"

	"github.com/ajromen/LSM-KV-Engine/internal/shared"
)

func TestFragment(t *testing.T) {
	tombstones := make([]shared.RangeDelEntry, 0)
	range1 := shared.RangeDelEntry{
		StartKey: []byte("a"),
		EndKey:   []byte("z"),
		SeqId:    10,
	}
	range2 := shared.RangeDelEntry{
		StartKey: []byte("c"),
		EndKey:   []byte("d"),
		SeqId:    4,
	}
	tombstones = append(tombstones, range1)
	tombstones = append(tombstones, range2)
	fragmented := FragmentRangeTombstones(tombstones)
	for _, fragmentedEntry := range fragmented {
		fmt.Printf("start key: %s end key: %s seqid: %d\n",
			fragmentedEntry.StartKey,
			fragmentedEntry.EndKey,
			fragmentedEntry.SeqId,
		)
	}
	fmt.Println(IsFragmented(fragmented))
	fmt.Println(IsFragmented(tombstones))
}
