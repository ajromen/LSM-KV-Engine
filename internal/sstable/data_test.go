package sstable

import (
	"strings"
	"testing"

	"github.com/ajromen/LSM-KV-Engine/internal/utils"
)

func ts(high, low uint64) utils.Uint128 {
	return utils.Uint128{
		High: high,
		Low:  low,
	}
}

func TestDataBlockBuilderBasic(t *testing.T) {
	t.Log("---- DATA BLOCK BUILDER BASIC TEST ----")
	builder := NewDataBlockBuilder(2, 1024)
	ok := builder.AddRecord(Record{
		Timestamp: ts(0, 1),
		Tombstone: false,
		Key:       []byte("key1"),
		Value:     []byte("value1"),
	})
	if !ok {
		t.Fatalf("expected record to be added")
	}
	ok = builder.AddRecord(Record{
		Timestamp: ts(0, 2),
		Tombstone: false,
		Key:       []byte("key2"),
		Value:     []byte("value2"),
	})
	if !ok {
		t.Fatalf("expected second record to be added")
	}
	if builder.RecordCount() != 2 {
		t.Fatalf("expected 2 records, got %d", builder.RecordCount())
	}
}

func TestDataBlockFinishAndRead(t *testing.T) {
	t.Log("---- DATA BLOCK FINISH AND READ TEST ----")
	builder := NewDataBlockBuilder(2, 1024)
	records := []Record{
		{Timestamp: ts(0, 1), Tombstone: false, Key: []byte("key1"), Value: []byte("value1")},
		{Timestamp: ts(0, 2), Tombstone: false, Key: []byte("key2"), Value: []byte("value2")},
		{Timestamp: ts(0, 3), Tombstone: true, Key: []byte("key3"), Value: nil},
	}
	for _, r := range records {
		ok := builder.AddRecord(r)
		if !ok {
			t.Fatalf("failed to add record %s", string(r.Key))
		}
	}
	block := builder.Finish(CompressionNone)
	VisualizeBlock(block)
	reader, err := NewDataBlockReader(block)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}
	i := 0
	for reader.HasNext() {
		rec, err := reader.Next()
		if err != nil {
			t.Fatalf("failed to read record: %v", err)
		}
		expected := records[i]
		if rec.Timestamp != expected.Timestamp {
			t.Fatalf(
				"expected timestamp %+v, got %+v",
				expected.Timestamp,
				rec.Timestamp,
			)
		}
		if rec.Tombstone != expected.Tombstone {
			t.Fatalf("expected tombstone %v, got %v", expected.Tombstone, rec.Tombstone)
		}
		if string(rec.Key) != string(expected.Key) {
			t.Fatalf("expected key %s, got %s", expected.Key, rec.Key)
		}
		if string(rec.Value) != string(expected.Value) {
			t.Fatalf("expected value %s, got %s", expected.Value, rec.Value)
		}
		i++
	}
	if i != len(records) {
		t.Fatalf("expected to read %d records, got %d", len(records), i)
	}
}

func TestDataBlockRestartSeek(t *testing.T) {
	t.Log("---- DATA BLOCK RESTART SEEK TEST ----")
	builder := NewDataBlockBuilder(1, 1024)
	builder.AddRecord(Record{Timestamp: ts(0, 1), Key: []byte("a"), Value: []byte("1")})
	builder.AddRecord(Record{Timestamp: ts(0, 2), Key: []byte("b"), Value: []byte("2")})
	builder.AddRecord(Record{Timestamp: ts(0, 3), Key: []byte("c"), Value: []byte("3")})
	block := builder.Finish(CompressionNone)
	reader, err := NewDataBlockReader(block)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}
	err = reader.SeekToRestart(0)
	if err != nil {
		t.Fatalf("failed to seek to restart: %v", err)
	}
	rec, err := reader.Next()
	if err != nil {
		t.Fatalf("failed to read after seek: %v", err)
	}
	if string(rec.Key) != "a" {
		t.Fatalf("expected key a after seek, got %s", rec.Key)
	}
}

func TestDataBlockCRCFailure(t *testing.T) {
	t.Log("---- DATA BLOCK CRC FAILURE TEST ----")
	builder := NewDataBlockBuilder(2, 1024)
	builder.AddRecord(Record{
		Timestamp: ts(0, 1),
		Key:       []byte("key"),
		Value:     []byte("value"),
	})
	block := builder.Finish(CompressionNone)
	block[10] ^= 0xff
	_, err := NewDataBlockReader(block)
	if err == nil {
		t.Fatalf("expected CRC mismatch error")
	}
}

func TestDataBlockReset(t *testing.T) {
	t.Log("---- DATA BLOCK RESET TEST ----")
	builder := NewDataBlockBuilder(2, 1024)
	builder.AddRecord(Record{
		Timestamp: ts(0, 1),
		Key:       []byte("key1"),
		Value:     []byte("value1"),
	})
	builder.Reset()
	if builder.RecordCount() != 0 {
		t.Fatalf("expected record count to be 0 after reset")
	}
	if builder.Size() != 0 {
		t.Fatalf("expected buffer size to be 0 after reset")
	}
}

func TestDataBlockVisualInspection(t *testing.T) {
	t.Log("==================================================")
	t.Log("     DATA BLOCK VISUAL INSPECTION TEST")
	t.Log("==================================================")
	restartInterval := 4
	blockSize := 2048
	builder := NewDataBlockBuilder(restartInterval, blockSize)
	t.Logf("\n Block Configuration:")
	t.Logf("   - Block Size: %d bytes", blockSize)
	t.Logf("   - Restart Interval: %d records", restartInterval)
	records := []Record{
		{Timestamp: ts(0, 1000), Tombstone: false, Key: []byte("user:001"), Value: []byte("alice")},
		{Timestamp: ts(0, 1001), Tombstone: false, Key: []byte("user:002"), Value: []byte("bob")},
		{Timestamp: ts(0, 1002), Tombstone: false, Key: []byte("user:003"), Value: []byte("charlie")},
		{Timestamp: ts(0, 1003), Tombstone: false, Key: []byte("user:004"), Value: []byte("diana")},
		{Timestamp: ts(0, 1004), Tombstone: false, Key: []byte("user:005"), Value: []byte("eve")},
		{Timestamp: ts(0, 1100), Tombstone: false, Key: []byte("product:apple"), Value: []byte("fruit:red:sweet")},
		{Timestamp: ts(0, 1101), Tombstone: false, Key: []byte("product:banana"), Value: []byte("fruit:yellow:soft")},
		{Timestamp: ts(0, 1102), Tombstone: true, Key: []byte("product:cherry"), Value: nil}, // Tombstone
		{Timestamp: ts(0, 1200), Tombstone: false, Key: []byte("score:alice"), Value: []byte("95")},
		{Timestamp: ts(0, 1201), Tombstone: false, Key: []byte("score:bob"), Value: []byte("87")},
		{Timestamp: ts(0, 1202), Tombstone: false, Key: []byte("score:charlie"), Value: []byte("92")},
		{Timestamp: ts(0, 1300), Tombstone: true, Key: []byte("temp:session:xyz"), Value: nil}, // Tombstone
		{Timestamp: ts(0, 1301), Tombstone: false, Key: []byte("config:max_users"), Value: []byte("1000")},
		{Timestamp: ts(0, 1302), Tombstone: false, Key: []byte("config:timeout"), Value: []byte("30s")},
		{Timestamp: ts(0, 1303), Tombstone: false, Key: []byte("config:debug"), Value: []byte("false")},
	}
	t.Logf("\n📝 Adding %d records to block:\n", len(records))
	for i, r := range records {
		ok := builder.AddRecord(r)
		if !ok {
			t.Fatalf("Failed to add record #%d: %s", i, string(r.Key))
		}
		tombstoneIcon := "✓"
		if r.Tombstone {
			tombstoneIcon = "🗑️"
		}
		valueStr := string(r.Value)
		if r.Tombstone {
			valueStr = "<deleted>"
		}
		restartMarker := ""
		if i%restartInterval == 0 {
			restartMarker = " RESTART POINT"
		}
		t.Logf("   [%2d] %s %-20s → %-20s (ts:%d)%s",
			i,
			tombstoneIcon,
			string(r.Key),
			valueStr,
			r.Timestamp.Low,
			restartMarker,
		)
	}
	t.Logf("\n  Block Statistics:")
	t.Logf("   - Records added: %d", builder.RecordCount())
	t.Logf("   - Data size (before metadata): %d bytes", builder.Size())
	block := builder.Finish(CompressionNone)
	t.Logf("   - Final block size: %d bytes", len(block))
	t.Logf("   - Space utilization: %.2f%%", float64(builder.Size())/float64(blockSize)*100)
	reader, err := NewDataBlockReader(block)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	t.Logf("\nReading back records:\n")
	i := 0
	for reader.HasNext() {
		rec, err := reader.Next()
		if err != nil {
			t.Fatalf("Failed to read record #%d: %v", i, err)
		}
		expected := records[i]
		if rec.Timestamp != expected.Timestamp {
			t.Fatalf("Record #%d: timestamp mismatch. Expected %+v, got %+v",
				i, expected.Timestamp, rec.Timestamp)
		}
		if rec.Tombstone != expected.Tombstone {
			t.Fatalf("Record #%d: tombstone mismatch. Expected %v, got %v",
				i, expected.Tombstone, rec.Tombstone)
		}
		if string(rec.Key) != string(expected.Key) {
			t.Fatalf("Record #%d: key mismatch. Expected %s, got %s",
				i, expected.Key, rec.Key)
		}
		if string(rec.Value) != string(expected.Value) {
			t.Fatalf("Record #%d: value mismatch. Expected %s, got %s",
				i, expected.Value, rec.Value)
		}
		tombstoneIcon := "✓"
		if rec.Tombstone {
			tombstoneIcon = "🗑️"
		}
		valueStr := string(rec.Value)
		if rec.Tombstone {
			valueStr = "<deleted>"
		}
		t.Logf("  [%2d] %s %-20s → %-20s",
			i,
			tombstoneIcon,
			string(rec.Key),
			valueStr,
		)
		i++
	}
	if i != len(records) {
		t.Fatalf("Expected to read %d records, got %d", len(records), i)
	}
	t.Logf("\nSuccessfully read all %d records", i)
	t.Logf("\nTesting Restart Point Seeking:")
	numRestarts := (len(records) + restartInterval - 1) / restartInterval
	if len(records)%restartInterval != 0 {
		numRestarts = len(records)/restartInterval + 1
	}
	for restartIdx := 0; restartIdx < numRestarts; restartIdx++ {
		expectedRecordIdx := restartIdx * restartInterval
		if expectedRecordIdx >= len(records) {
			break
		}
		err := reader.SeekToRestart(restartIdx)
		if err != nil {
			t.Fatalf("Failed to seek to restart point %d: %v", restartIdx, err)
		}
		rec, err := reader.Next()
		if err != nil {
			t.Fatalf("Failed to read after seeking to restart %d: %v", restartIdx, err)
		}
		expected := records[expectedRecordIdx]
		if string(rec.Key) != string(expected.Key) {
			t.Fatalf("After seeking to restart %d, expected key %s, got %s",
				restartIdx, expected.Key, rec.Key)
		}
		t.Logf(" Restart #%d → Record #%d: %s ✅",
			restartIdx,
			expectedRecordIdx,
			string(rec.Key),
		)
	}
	t.Logf("\nDelta Encoding Efficiency Analysis:")
	prefixGroups := make(map[string]int)
	for _, r := range records {
		key := string(r.Key)
		if len(key) > 0 {
			prefix := key
			if idx := strings.Index(key, ":"); idx != -1 {
				prefix = key[:idx+1]
			} else if len(key) > 5 {
				prefix = key[:5]
			}
			prefixGroups[prefix]++
		}
	}
	totalOriginalSize := 0
	for _, r := range records {
		totalOriginalSize += len(r.Key) + len(r.Value)
	}
	t.Logf("   - Original keys+values size: %d bytes", totalOriginalSize)
	t.Logf("   - Actual data in block: %d bytes", builder.Size())
	t.Logf("   - Compression ratio: %.2fx", float64(totalOriginalSize)/float64(builder.Size()))
	t.Logf("   - Key prefix groups:")
	for prefix, count := range prefixGroups {
		if count > 1 {
			t.Logf("     • '%s': %d keys (delta encoding saves ~%d bytes)",
				prefix, count, (count-1)*len(prefix))
		}
	}
	t.Log("\n==================================================")
	t.Log("      ALL TESTS PASSED")
	t.Log("==================================================")
}

func TestDataBlockIteratorForwardScan(t *testing.T) {
	t.Log("---- DATA BLOCK ITERATOR TEST ----")
	builder := NewDataBlockBuilder(2, 1024)
	records := []Record{
		{Timestamp: ts(0, 1), Key: []byte("a"), Value: []byte("1")},
		{Timestamp: ts(0, 2), Key: []byte("b"), Value: []byte("2")},
		{Timestamp: ts(0, 3), Key: []byte("c"), Value: []byte("3")},
	}
	for _, r := range records {
		builder.AddRecord(r)
	}
	block := builder.Finish(CompressionNone)
	it, err := NewDataBlockIterator(block)
	if err != nil {
		t.Fatalf("failed to create iterator: %v", err)
	}
	for i := 0; i < len(records); i++ {
		if !it.Valid() {
			t.Fatalf("iterator invalid at index %d", i)
		}
		if string(it.Key()) != string(records[i].Key) {
			t.Fatalf("expected key %s, got %s", records[i].Key, it.Key())
		}
		it.Next()
	}
	if it.Valid() {
		t.Fatalf("iterator should be invalid after last record")
	}
}

func TestDataBlockIteratorSeekExact(t *testing.T) {
	t.Log("---- DATA BLOCK ITERATOR SEEK TEST (IF KEY EXISTS) ----")
	builder := NewDataBlockBuilder(1, 1024)
	keys := []string{"a", "b", "c", "d"}
	for i, k := range keys {
		builder.AddRecord(Record{
			Timestamp: ts(0, uint64(i)),
			Key:       []byte(k),
			Value:     []byte("v"),
		})
	}
	block := builder.Finish(CompressionNone)
	it, _ := NewDataBlockIterator(block)

	err := it.Seek([]byte("c"))
	if err != nil {
		t.Fatalf("seek failed: %v", err)
	}
	if !it.Valid() {
		t.Fatalf("iterator invalid after seek")
	}
	if string(it.Key()) != "c" {
		t.Fatalf("expected key c, got %s", it.Key())
	}
}

func TestDataBlockIteratorSeekBetweenKeys(t *testing.T) {
	t.Log("---- DATA BLOCK ITERATOR SEEK TEST (IF KEY DOESNT EXIST) ----")
	builder := NewDataBlockBuilder(2, 1024)
	keys := []string{"a", "c", "e"}
	for i, k := range keys {
		builder.AddRecord(Record{
			Timestamp: ts(0, uint64(i)),
			Key:       []byte(k),
			Value:     []byte("v"),
		})
	}
	block := builder.Finish(CompressionNone)
	it, _ := NewDataBlockIterator(block)
	err := it.Seek([]byte("b"))
	if err != nil {
		t.Fatalf("seek failed: %v", err)
	}
	if !it.Valid() {
		t.Fatalf("iterator invalid after seek")
	}
	if string(it.Key()) != "c" {
		t.Fatalf("expected key c, got %s", it.Key())
	}
}

func TestDataBlockIteratorSeekPastEnd(t *testing.T) {
	t.Log("---- DATA BLOCK ITERATOR SEEK TEST PAST THE END ----")
	builder := NewDataBlockBuilder(2, 1024)
	builder.AddRecord(Record{Timestamp: ts(0, 1), Key: []byte("a"), Value: []byte("1")})
	builder.AddRecord(Record{Timestamp: ts(0, 2), Key: []byte("b"), Value: []byte("2")})
	block := builder.Finish(CompressionNone)
	it, _ := NewDataBlockIterator(block)
	err := it.Seek([]byte("z"))
	if err != nil {
		t.Fatalf("seek failed: %v", err)
	}
	if it.Valid() {
		t.Fatalf("iterator should be invalid after seek past end")
	}
}

func TestDataBlockIteratorRewindAfterSeek(t *testing.T) {
	t.Log("---- DATA BLOCK ITERATOR REWIND AFTER SEEK TEST ----")
	builder := NewDataBlockBuilder(1, 1024)
	builder.AddRecord(Record{Timestamp: ts(0, 1), Key: []byte("a"), Value: []byte("1")})
	builder.AddRecord(Record{Timestamp: ts(0, 2), Key: []byte("b"), Value: []byte("2")})
	block := builder.Finish(CompressionNone)
	it, _ := NewDataBlockIterator(block)
	it.Seek([]byte("b"))
	if string(it.Key()) != "b" {
		t.Fatalf("expected key b after seek")
	}
	it.Rewind()
	if !it.Valid() {
		t.Fatalf("iterator invalid after rewind")
	}
	if string(it.Key()) != "a" {
		t.Fatalf("expected key a after rewind, got %s", it.Key())
	}
}

func TestMergeIteratorSimpleMerge(t *testing.T) {
	t.Log("---- MERGE ITERATOR SIMPLE MERGE TEST ----")
	builder1 := NewDataBlockBuilder(1, 1024)
	builder1.AddRecord(Record{Timestamp: ts(0, 1), Key: []byte("a"), Value: []byte("1")})
	builder1.AddRecord(Record{Timestamp: ts(0, 2), Key: []byte("c"), Value: []byte("3")})
	builder2 := NewDataBlockBuilder(1, 1024)
	builder2.AddRecord(Record{Timestamp: ts(0, 1), Key: []byte("b"), Value: []byte("2")})
	builder2.AddRecord(Record{Timestamp: ts(0, 2), Key: []byte("d"), Value: []byte("4")})
	it1, _ := NewDataBlockIterator(builder1.Finish(CompressionNone))
	it2, _ := NewDataBlockIterator(builder2.Finish(CompressionNone))
	merge := NewMergeIterator(it1, it2)
	expectedKeys := []string{"a", "b", "c", "d"}
	i := 0
	for merge.Valid() {
		if string(merge.Key()) != expectedKeys[i] {
			t.Fatalf("expected key %s, got %s", expectedKeys[i], merge.Key())
		}
		i++
		merge.Next()
	}
	if i != len(expectedKeys) {
		t.Fatalf("expected %d keys, got %d", len(expectedKeys), i)
	}
}

func TestMergeIteratorSameKeyPickNewestTimestamp(t *testing.T) {
	t.Log("---- MERGE ITERATOR SAME KEY TIMESTAMP TEST ----")
	builder1 := NewDataBlockBuilder(1, 1024)
	builder1.AddRecord(Record{
		Timestamp: ts(0, 10),
		Key:       []byte("a"),
		Value:     []byte("old"),
	})
	builder2 := NewDataBlockBuilder(1, 1024)
	builder2.AddRecord(Record{
		Timestamp: ts(0, 20),
		Key:       []byte("a"),
		Value:     []byte("new"),
	})
	it1, _ := NewDataBlockIterator(builder1.Finish(CompressionNone))
	it2, _ := NewDataBlockIterator(builder2.Finish(CompressionNone))
	merge := NewMergeIterator(it1, it2)
	if !merge.Valid() {
		t.Fatalf("merge iterator invalid")
	}
	if string(merge.Key()) != "a" {
		t.Fatalf("expected key a, got %s", merge.Key())
	}
	if string(merge.Value()) != "new" {
		t.Fatalf("expected value 'new', got %s", merge.Value())
	}
	merge.Next()
	if merge.Valid() {
		t.Fatalf("merge iterator should be exhausted")
	}
}

func emptyIterator(t *testing.T) *DataBlockIterator {
	builder := NewDataBlockBuilder(1, 1024)
	block := builder.Finish(CompressionNone)
	it, err := NewDataBlockIterator(block)
	if err != nil {
		t.Fatalf("failed to create empty iterator: %v", err)
	}
	return it
}

func TestMergeIteratorOneEmpty(t *testing.T) {
	t.Log("---- MERGE ITERATOR ONE EMPTY TEST ----")
	builder := NewDataBlockBuilder(1, 1024)
	builder.AddRecord(Record{Timestamp: ts(0, 1), Key: []byte("a"), Value: []byte("1")})
	builder.AddRecord(Record{Timestamp: ts(0, 2), Key: []byte("b"), Value: []byte("2")})
	it1, _ := NewDataBlockIterator(builder.Finish(CompressionNone))
	it2 := emptyIterator(t)
	merge := NewMergeIterator(it1, it2)
	if !merge.Valid() {
		t.Fatalf("merge iterator invalid")
	}
	if string(merge.Key()) != "a" {
		t.Fatalf("expected key a, got %s", merge.Key())
	}
}

func TestMergeIteratorTombstoneWinsByTimestamp(t *testing.T) {
	t.Log("---- MERGE ITERATOR TOMBSTONE TEST ----")
	builder1 := NewDataBlockBuilder(1, 1024)
	builder1.AddRecord(Record{
		Timestamp: ts(0, 5),
		Key:       []byte("a"),
		Value:     []byte("alive"),
	})
	builder2 := NewDataBlockBuilder(1, 1024)
	builder2.AddRecord(Record{
		Timestamp: ts(0, 10),
		Key:       []byte("a"),
		Tombstone: true,
	})
	it1, _ := NewDataBlockIterator(builder1.Finish(CompressionNone))
	it2, _ := NewDataBlockIterator(builder2.Finish(CompressionNone))
	merge := NewMergeIterator(it1, it2)
	if !merge.Tombstone() {
		t.Fatalf("expected tombstone to win")
	}
}

func TestMergeIteratorExhaustion(t *testing.T) {
	t.Log("---- MERGE ITERATOR EXHAUSTION TEST ----")
	builder := NewDataBlockBuilder(1, 1024)
	builder.AddRecord(Record{Timestamp: ts(0, 1), Key: []byte("a"), Value: []byte("1")})
	it1, _ := NewDataBlockIterator(builder.Finish(CompressionNone))
	it2, _ := NewDataBlockIterator(builder.Finish(CompressionNone))
	merge := NewMergeIterator(it1, it2)
	if !merge.Valid() {
		t.Fatalf("merge iterator invalid initially")
	}
	merge.Next()
	if merge.Valid() {
		t.Fatalf("merge iterator should be invalid after consuming all")
	}
}
