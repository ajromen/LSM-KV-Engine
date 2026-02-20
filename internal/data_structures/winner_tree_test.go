package data_structures

import (
	"fmt"
	"testing"

	"github.com/ajromen/LSM-KV-Engine/internal/sstable"
	"github.com/ajromen/LSM-KV-Engine/internal/utils"
)

func TestWinnerTreeSortedMerge(t *testing.T) {
	builder1 := sstable.NewDataBlockBuilder(2, 128)
	for _, r := range []sstable.Record{
		{Timestamp: utils.Uint128{High: 0, Low: 0}, Key: []byte("key1"), Value: []byte("val1")},
		{Timestamp: utils.Uint128{High: 0, Low: 0}, Key: []byte("key3"), Value: []byte("val3")},
		{Timestamp: utils.Uint128{High: 0, Low: 0}, Key: []byte("key5"), Value: []byte("val5")},
	} {
		builder1.AddRecord(r)
	}

	builder2 := sstable.NewDataBlockBuilder(2, 128)
	for _, r := range []sstable.Record{
		{Timestamp: utils.Uint128{High: 0, Low: 0}, Key: []byte("key2"), Value: []byte("val2")},
		{Timestamp: utils.Uint128{High: 0, Low: 0}, Key: []byte("key4"), Value: []byte("val4")},
		{Timestamp: utils.Uint128{High: 0, Low: 0}, Key: []byte("key6"), Value: []byte("val6")},
	} {
		builder2.AddRecord(r)
	}

	builder3 := sstable.NewDataBlockBuilder(2, 128)
	for _, r := range []sstable.Record{
		{Timestamp: utils.Uint128{High: 0, Low: 0}, Key: []byte("key7"), Value: []byte("val7")},
		{Timestamp: utils.Uint128{High: 0, Low: 0}, Key: []byte("key9"), Value: []byte("val9")},
		{Timestamp: utils.Uint128{High: 0, Low: 0}, Key: []byte("key10"), Value: []byte("val10")},
	} {
		builder3.AddRecord(r)
	}

	block1, _ := builder1.Finish(128)
	block2, _ := builder2.Finish(128)
	block3, _ := builder3.Finish(128)

	it1, _ := sstable.NewDataBlockIterator(block1)
	it2, _ := sstable.NewDataBlockIterator(block2)
	it3, _ := sstable.NewDataBlockIterator(block3)
	it1.Rewind()
	it2.Rewind()
	it3.Rewind()

	wt := NewWinnerTree([]*sstable.DataBlockIterator{it1, it2, it3})
	if wt == nil {
		t.Fatal("NewWinnerTree returned nil")
	}

	expected := []string{
		"key1", "key2", "key3", "key4", "key5", "key6", "key7", "key9", "key10",
	}

	var got []string
	for {
		winner := wt.Winner()
		if winner == nil || !winner.Valid() {
			break
		}
		got = append(got, string(winner.Key()))
		wt.Update(WinnerTreeNode{iterator: winner})
	}

	fmt.Println("Merged output:", got)

	if len(got) != len(expected) {
		t.Fatalf("expected %d records, got %d", len(expected), len(got))
	}
	for i, key := range expected {
		if got[i] != key {
			t.Errorf("position %d: expected %q, got %q", i, key, got[i])
		}
	}
	fmt.Println("All keys merged in sorted order!")
}
