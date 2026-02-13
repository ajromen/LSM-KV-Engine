package sstable

import (
	"fmt"
	"testing"

	"github.com/ajromen/LSM-KV-Engine/internal/utils"
)

func makeRecord(key, value string, high, low uint64, tomb bool) Record {
	return Record{
		Timestamp: utils.Uint128{High: high, Low: low},
		Tombstone: tomb,
		Key:       []byte(key),
		Value:     []byte(value),
	}
}

func buildTestBlock(t *testing.T) []byte {
	builder := NewDataBlockBuilder(2, 512)

	records := []Record{
		makeRecord("key1", "val1", 1, 1, false),
		makeRecord("key2", "val2", 2, 2, false),
		makeRecord("key3", "val3", 3, 3, false),
		makeRecord("key4", "val4", 4, 4, true),
	}

	for _, r := range records {
		ok := builder.AddRecord(r)
		if !ok {
			t.Fatal("AddRecord failed unexpectedly")
		}
	}

	block, err := builder.Finish(512)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(block)
	reader, err := NewDataBlockReader(block)
	record1, err := reader.ReadRecord()
	if err != nil {
		t.Fatal(err)
	}
	print(record1.Key)
	return block
}

func TestFinishEmptyBlock(t *testing.T) {
	builder := NewDataBlockBuilder(2, 128)
	_, err := builder.Finish(128)
	if err == nil {
		t.Fatal("Expected error for empty block")
	}
}

func TestReset(t *testing.T) {
	builder := NewDataBlockBuilder(2, 128)
	builder.AddRecord(makeRecord("a", "b", 1, 1, false))
	builder.Reset()

	if builder.recordCount != 0 {
		t.Fatal("Reset did not reset recordCount")
	}
	if builder.firstKey != nil {
		t.Fatal("Reset did not clear firstKey")
	}
}

func TestReaderSequentialRead(t *testing.T) {
	block := buildTestBlock(t)

	reader, err := NewDataBlockReader(block)
	if err != nil {
		t.Fatal(err)
	}

	count := 0
	for reader.HasNext() {
		_, err := reader.ReadRecord()
		if err != nil {
			t.Fatal(err)
		}
		count++
	}

	if count != 4 {
		t.Fatalf("Expected 4 records got %d", count)
	}
}

func TestCRCFailure(t *testing.T) {
	block := buildTestBlock(t)
	block[10] ^= 0xFF // corrupt

	_, err := NewDataBlockReader(block)
	if err == nil {
		t.Fatal("Expected CRC mismatch error")
	}
}

func TestIterator(t *testing.T) {
	builder := NewDataBlockBuilder(2, 128)
	records := []Record{
		Record{Timestamp: utils.Uint128{High: 0, Low: 0}, Tombstone: false, Key: []byte("key1"), Value: []byte("value")},
		Record{Timestamp: utils.Uint128{High: 0, Low: 0}, Tombstone: false, Key: []byte("key2"), Value: []byte("value")},
		Record{Timestamp: utils.Uint128{High: 0, Low: 0}, Tombstone: false, Key: []byte("key3"), Value: []byte("value")},
		Record{Timestamp: utils.Uint128{High: 0, Low: 0}, Tombstone: false, Key: []byte("key4"), Value: []byte("value")},
	}
	for _, record := range records {
		added := builder.AddRecord(record)
		if added != true {
			t.Fatal("AddRecord failed unexpectedly")
		}
	}
	block, err := builder.Finish(128)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := NewDataBlockReader(block)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(block)
	record1, _ := reader.ReadRecord()
	record2, _ := reader.ReadRecord()
	record3, _ := reader.ReadRecord()
	record4, _ := reader.ReadRecord()
	fmt.Println(string(record1.Key), string(record2.Key), string(record3.Key), string(record4.Key))
	reader.Restart()
	iterator, err := NewDataBlockIterator(block)
	if err != nil {
		t.Fatal(err)
	}
	err = iterator.Rewind()
	if err != nil {
		t.Fatal("Rewinding iterator failed unexpectedly")
	}
	fmt.Println(string(iterator.Current().Key))
	err = iterator.Next()
	if err != nil {
		t.Fatal("Iterator failed unexpectedly")
	}
	fmt.Println(string(iterator.Current().Key))
	err = iterator.Next()
	if err != nil {
		t.Fatal("Iterator failed unexpectedly")
	}
	fmt.Println(string(iterator.Current().Key))
	err = iterator.Next()
	if err != nil {
		t.Fatal("Iterator failed unexpectedly")
	}
}

func TestIteratorSeek(t *testing.T) {
	builder := NewDataBlockBuilder(2, 128)
	records := []Record{
		{Timestamp: utils.Uint128{High: 0, Low: 0}, Tombstone: false, Key: []byte("key1"), Value: []byte("value")},
		{Timestamp: utils.Uint128{High: 0, Low: 0}, Tombstone: false, Key: []byte("key2"), Value: []byte("value")},
		{Timestamp: utils.Uint128{High: 0, Low: 0}, Tombstone: false, Key: []byte("key3"), Value: []byte("value")},
		{Timestamp: utils.Uint128{High: 0, Low: 0}, Tombstone: false, Key: []byte("key4"), Value: []byte("value")},
	}
	for _, record := range records {
		added := builder.AddRecord(record)
		if !added {
			t.Fatal("AddRecord failed unexpectedly")
		}
	}
	block, err := builder.Finish(128)
	if err != nil {
		t.Fatal(err)
	}
	iterator, err := NewDataBlockIterator(block)
	if err != nil {
		t.Fatal(err)
	}
	targetKeys := [][]byte{
		[]byte("key1"),
		[]byte("key2"),
		[]byte("key3"),
		[]byte("key4"),
		[]byte("key5"),
	}
	for _, target := range targetKeys {
		fmt.Printf("Seeking for: %s\n", target)
		err := iterator.Seek(target)
		if err != nil {
			fmt.Printf("Seek returned error: %v\n", err)
			continue
		}
		if iterator.Valid() {
			fmt.Printf("Found key: %s, value: %s\n", iterator.Current().Key, iterator.Current().Value)
			for iterator.Next() != nil {
				fmt.Printf("Next key: %s, value: %s\n", iterator.Current().Key, iterator.Current().Value)
			}
		} else {
			fmt.Println("Key not found")
		}
		fmt.Println("--------")
		iterator.Rewind()
	}
}

func TestMergeIterator(t *testing.T) {
	builder1 := NewDataBlockBuilder(2, 128)
	records1 := []Record{
		{Timestamp: utils.Uint128{High: 0, Low: 0}, Tombstone: false, Key: []byte("key1"), Value: []byte("val1")},
		{Timestamp: utils.Uint128{High: 0, Low: 0}, Tombstone: false, Key: []byte("key3"), Value: []byte("val3")},
		{Timestamp: utils.Uint128{High: 0, Low: 0}, Tombstone: false, Key: []byte("key5"), Value: []byte("val5")},
	}
	for _, r := range records1 {
		if !builder1.AddRecord(r) {
			t.Fatal("AddRecord failed for builder1")
		}
	}
	builder2 := NewDataBlockBuilder(2, 128)
	records2 := []Record{
		{Timestamp: utils.Uint128{High: 0, Low: 0}, Tombstone: false, Key: []byte("key2"), Value: []byte("val2")},
		{Timestamp: utils.Uint128{High: 0, Low: 0}, Tombstone: false, Key: []byte("key4"), Value: []byte("val4")},
		{Timestamp: utils.Uint128{High: 0, Low: 0}, Tombstone: false, Key: []byte("key6"), Value: []byte("val6")},
	}
	for _, r := range records2 {
		if !builder2.AddRecord(r) {
			t.Fatal("AddRecord failed for builder2")
		}
	}
	block1, _ := builder1.Finish(128)
	block2, _ := builder2.Finish(128)
	iterator1, _ := NewDataBlockIterator(block1)
	iterator2, _ := NewDataBlockIterator(block2)
	iterator1.Rewind()
	iterator2.Rewind()
	mergeIterator := NewMergeIterator(iterator1, iterator2)
	expectedKeys := []string{"key1", "key2", "key3", "key4", "key5", "key6"}
	idx := 0
	for mergeIterator.Valid() {
		key := string(mergeIterator.Key())
		if key != expectedKeys[idx] {
			t.Fatalf("Expected key %s but got %s", expectedKeys[idx], key)
		}
		fmt.Println("MergeIterator key:", key)
		mergeIterator.Next()
		idx++
	}
	if idx != len(expectedKeys) {
		t.Fatalf("Expected to iterate %d keys but got %d", len(expectedKeys), idx)
	}
}
