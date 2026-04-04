package wal

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
)

// to do : Recovery, Batch/transactions, Sync policy, Low WaterMark
// 1.1 Write-Ahead Log (WAL)
// WAL treba implementirati kao segmentirani log.
// Svaki segment ima fiksan broj zapisa koje korisnik specificira.
// ????
// ne sece rekord po segmentima kako treba

const (
	FilePrefix = "wal_"
	FileSuffix = ".log"
)

type WAL struct {
	Dir           string
	ActiveSegment *Segment
	BlockSize     int
	MaxBlocks     int
	NextSegmentID uint64
	BM            *block.BlockManager
}

func OpenWAL(dir string, blockSize int, maxBlocks int) (*WAL, error) {
	if dir == "" {
		return nil, fmt.Errorf("wal dir is empty")
	}
	if blockSize < KEY_START+1 {
		return nil, fmt.Errorf("blockSize is smaller than minimum WAL fragment size")
	}
	if maxBlocks <= 0 {
		return nil, fmt.Errorf("maxBlocks must be >0")
	}
	err := block.EnsureDir(dir)
	if err != nil {
		return nil, err
	}
	bm := block.NewBlockManager(blockSize)

	w := &WAL{
		Dir:           dir,
		BlockSize:     blockSize,
		MaxBlocks:     maxBlocks,
		NextSegmentID: 1,
		BM:            bm,
	}

	entries, err := ListFiles(dir)
	if err != nil {
		return nil, err
	}
	maxID := uint64(0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, FilePrefix) || !strings.HasSuffix(name, FileSuffix) {
			continue
		}
		id, err := ParseSegmentID(name)
		if err != nil {
			return nil, err
		}
		if id > maxID {
			maxID = id
		}

	}

	if maxID == 0 { // id no log files
		firstPath := w.SegmentPath(w.NextSegmentID)
		seg, err := OpenSegment(w.NextSegmentID, firstPath, w.MaxBlocks, w.BM)
		if err != nil {
			return nil, err
		}
		w.ActiveSegment = seg
		w.NextSegmentID++

		return w, nil
	}

	// load last log file by id
	lastPath := w.SegmentPath(maxID)
	seg, err := OpenSegment(maxID, lastPath, w.MaxBlocks, w.BM)
	if err != nil {
		return nil, err
	}

	w.ActiveSegment = seg
	w.NextSegmentID = maxID + 1

	if w.ActiveSegment.IsFull() {
		err = w.RotateSegment()
		if err != nil {
			return nil, err
		}
	}

	return w, nil

}

func (w *WAL) Append(r Record) error {
	if w == nil {
		return fmt.Errorf("wal is nil")
	}
	if w.ActiveSegment == nil {
		return fmt.Errorf("active segment is nil")
	}
	err := w.ActiveSegment.Append(r)
	if err == nil {
		return nil
	}
	if !w.ActiveSegment.IsFull() {
		return err
	}

	err = w.RotateSegment()
	if err != nil {
		return err
	}

	return w.ActiveSegment.Append(r)
}

func (w *WAL) Put(key []byte, value []byte, timestamp uint64) error {
	r := Record{
		Timestamp: timestamp,
		Tombstone: false,
		Key:       key,
		Value:     value,
	}
	err := w.Append(r)
	if err != nil {
		return err
	}
	return nil
}

func (w *WAL) Delete(key []byte, timestamp uint64) error {
	r := Record{
		Timestamp: timestamp,
		Tombstone: true,
		Key:       key,
		Value:     nil,
	}
	err := w.Append(r)
	if err != nil {
		return err
	}
	return nil
}

func (w *WAL) Recover() ([]Record, error) { // doesnt call memtable, just returns list of records
	frags, err := w.ReadAllFragments()
	if err != nil {
		return nil, err
	}

	records, err := JoinFragments(frags)
	if err != nil {
		return nil, err
	}
	return records, nil
}

func (w *WAL) ReadAllFragments() ([]WALRecord, error) {
	if w == nil {
		return nil, fmt.Errorf("wal is nil")
	}

	entries, err := ListFiles(w.Dir)
	if err != nil {
		return nil, err
	}

	type segInfo struct {
		id   uint64
		path string
	}

	segments := make([]segInfo, 0)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, FilePrefix) || !strings.HasSuffix(name, FileSuffix) {
			continue
		}

		id, err := ParseSegmentID(name)
		if err != nil {
			return nil, err
		}

		segments = append(segments, segInfo{
			id:   id,
			path: filepath.Join(w.Dir, name),
		})
	}

	for i := 0; i < len(segments); i++ {
		for j := i + 1; j < len(segments); j++ {
			if segments[j].id < segments[i].id {
				temp := segments[j]
				segments[j] = segments[i]
				segments[i] = temp
			}
		}
	}

	all := make([]WALRecord, 0)

	for _, segInfo := range segments {
		seg, err := OpenSegment(segInfo.id, segInfo.path, w.MaxBlocks, w.BM)
		if err != nil {
			return nil, err
		}

		recs, err := seg.ReadAllRecords()
		if err != nil {
			return nil, err
		}

		all = append(all, recs...)
	}

	return all, nil
}

func JoinFragments(frags []WALRecord) ([]Record, error) { // returns error for last unfinished record, should ignore it!
	records := make([]Record, 0)

	var current *Record
	inFragment := false

	for _, frag := range frags {
		switch frag.FragType {
		case FULL:
			records = append(records, frag.Record)
		case FIRST:
			if inFragment {
				return nil, fmt.Errorf("found FIRST before previous fragmented record was finished")
			}

			rec := Record{
				Timestamp: frag.Record.Timestamp,
				Tombstone: frag.Record.Tombstone,
				Key:       append([]byte(nil), frag.Record.Key...),
				Value:     append([]byte(nil), frag.Record.Value...),
			}
			current = &rec
			inFragment = true

		case MIDDLE:
			if !inFragment || current == nil {
				return nil, fmt.Errorf("found MIDDLE without active fragmented record")
			}

			current.Key = append(current.Key, frag.Record.Key...)
			current.Value = append(current.Value, frag.Record.Value...)

		case LAST:
			if !inFragment || current == nil {
				return nil, fmt.Errorf("found LAST without active fragmented record")
			}
			current.Key = append(current.Key, frag.Record.Key...)
			current.Value = append(current.Value, frag.Record.Value...)

			records = append(records, *current)
			current = nil
			inFragment = false
		default:
			return nil, fmt.Errorf("invalid frag type")

		}

	}
	if inFragment { // should probably ignore last unfinished fragmented record?
		return nil, fmt.Errorf("unfinished fragmented record at the end of WAL")
	}

	return records, nil

}

func (w *WAL) RotateSegment() error {
	if w == nil {
		return fmt.Errorf("wal is nil")
	}
	if w.ActiveSegment == nil {
		return fmt.Errorf("active segment is nil")
	}
	err := w.ActiveSegment.Sync()
	if err != nil {
		return err
	}

	newPath := w.SegmentPath(w.NextSegmentID)
	seg, err := OpenSegment(w.NextSegmentID, newPath, w.MaxBlocks, w.BM)
	if err != nil {
		return err
	}
	w.ActiveSegment = seg
	w.NextSegmentID++
	return nil
}

func (w *WAL) Sync() error {
	if w == nil {
		return fmt.Errorf("wal is nil")
	}
	if w.ActiveSegment == nil {
		return fmt.Errorf("active segment is nil")
	}
	return w.ActiveSegment.Sync()
}

func (w *WAL) PrintAll() error { //func for debugging
	id := uint(1)
	for {
		path := w.SegmentPath(uint64(id))
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			break
		}
		s, err := OpenSegment(uint64(id), w.SegmentPath(uint64(id)), w.MaxBlocks, w.BM)
		if err != nil {
			break
		}
		bindex := 0
		//fmt.Println("Segment", id)
		for {
			block, err := s.ReadBlock(uint32(bindex))
			if err != nil {
				break
			}
			fmt.Println(block)
			bindex++
		}
		id++

	}
	return nil
}

func ParseSegmentID(name string) (uint64, error) {
	base := strings.TrimPrefix(name, FilePrefix)
	base = strings.TrimSuffix(base, FileSuffix)
	//fmt.Println(name, base)

	id, err := strconv.ParseUint(base, 10, 64)
	if err != nil {
		return 0, err
	}
	return id, nil

}

func (w *WAL) SegmentPath(id uint64) string {
	filename := fmt.Sprintf("wal_%06d.log", id)
	return filepath.Join(w.Dir, filename)
}

// helper function, should go to block manager
func ListFiles(dir string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	return entries, err
}
