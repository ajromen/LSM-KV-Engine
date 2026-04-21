package wal

import (
	"encoding/binary"
	"fmt"
	"os"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
)

type Segment struct {
	ID                uint64
	Path              string
	BlockSize         int
	MaxBlocks         int
	CurrentBlockIndex uint32
	CurrentBlock      *Block
	BM                *block.BlockManager
}

func OpenSegment(id uint64, path string, maxBlocks int, bm *block.BlockManager) (*Segment, error) {
	if path == "" {
		return nil, fmt.Errorf("path is empty")
	}
	if bm == nil {
		return nil, fmt.Errorf("block manager is nil")
	}
	if maxBlocks <= 0 {
		return nil, fmt.Errorf("maxBlocks must be >0")
	}
	if bm.BlockSize() < KEY_START+1 {
		return nil, fmt.Errorf("blockSize is smaller than minimum WAL fragment size")
	}
	exists, err := FileExists(path)
	if err != nil {
		return nil, err
	}
	if exists {
		s := Segment{
			ID:        id,
			Path:      path,
			BlockSize: bm.BlockSize(),
			MaxBlocks: maxBlocks,
			BM:        bm,
		}
		err := bm.EnsureSize(path, int64(bm.BlockSize()*maxBlocks))
		if err != nil {
			return nil, err
		}
		lastUsedIndex := uint32(0)
		lastWritePos := 0
		foundAny := false

		for i := uint32(0); i < uint32(maxBlocks); i++ {
			blockData, err := s.ReadBlock(i)
			if err != nil {
				return nil, err
			}
			writePos, hasData, err := ScanBlock(blockData)
			if err != nil {
				return nil, err
			}
			if !hasData {
				break
			}

			foundAny = true
			lastUsedIndex = i
			lastWritePos = writePos
		}

		if !foundAny {
			s.CurrentBlockIndex = 0
			s.CurrentBlock = NewBlock(bm.BlockSize())
			return &s, nil
		}

		blockData, err := s.ReadBlock(lastUsedIndex)
		if err != nil {
			return nil, err
		}

		s.CurrentBlockIndex = lastUsedIndex
		s.CurrentBlock = &Block{
			Data:     blockData,
			Size:     bm.BlockSize(),
			WritePos: lastWritePos,
		}

		return &s, nil

	} else {
		s := Segment{
			ID:                id,
			Path:              path,
			BlockSize:         bm.BlockSize(),
			MaxBlocks:         maxBlocks,
			CurrentBlockIndex: 0,
			CurrentBlock:      NewBlock(bm.BlockSize()),
			BM:                bm,
		}
		err := bm.EnsureSize(path, int64(bm.BlockSize()*maxBlocks))
		if err != nil {
			return nil, err
		}
		return &s, nil
	}
}

func (s *Segment) Append(r Record, sequenceID uint64) error {
	headerSize := KEY_START
	totalSize := headerSize + len(r.Key) + len(r.Value)
	if s.CurrentBlock.Remaining() >= totalSize { // if full record can fit
		wr := WALRecord{
			FragType:   FULL,
			RecType:    SINGLE, // to be updated
			TxnID:      0,      // to be updated
			SequenceID: sequenceID,
			KeySize:    uint64(len(r.Key)),
			ValueSize:  uint64(len(r.Value)),
			Record:     r,
		}
		buf := Encode(wr)

		_, err := s.CurrentBlock.Write(buf)
		if err != nil {
			return err
		}
		return nil

	} else if s.CurrentBlock.Remaining() >= headerSize+1 { // full header can fit and at least a byte of payload
		wr := WALRecord{}
		payloadSize := len(r.Key) + len(r.Value)
		payload := make([]byte, payloadSize)

		pValueStart := len(r.Key)
		copy(payload[:pValueStart], r.Key)
		copy(payload[pValueStart:], r.Value)

		payloadSpace := s.CurrentBlock.Remaining() - headerSize

		if payloadSpace < pValueStart { // part of key fits
			wr = WALRecord{
				FragType:   FIRST,
				RecType:    SINGLE, // to be updated
				TxnID:      0,      // to be updated
				SequenceID: sequenceID,
				KeySize:    uint64(payloadSpace),
				ValueSize:  0,
				Record: Record{
					Timestamp: r.Timestamp,
					Tombstone: r.Tombstone,
					Key:       payload[:payloadSpace],
					Value:     nil,
				},
			}
			payload = payload[payloadSpace:]
			pValueStart = pValueStart - payloadSpace
		} else if payloadSpace == pValueStart { // exactly key fits
			wr = WALRecord{
				FragType:   FIRST,
				RecType:    SINGLE, // to be updated
				TxnID:      0,      // to be updated
				SequenceID: sequenceID,
				KeySize:    uint64(pValueStart),
				ValueSize:  0,
				Record: Record{
					Timestamp: r.Timestamp,
					Tombstone: r.Tombstone,
					Key:       payload[:pValueStart],
					Value:     nil,
				},
			}
			payload = payload[pValueStart:]
			pValueStart = 0
		} else { // key and part of value fits
			wr = WALRecord{
				FragType:   FIRST,
				RecType:    SINGLE, // to be updated
				TxnID:      0,      // to be updated
				SequenceID: sequenceID,
				KeySize:    uint64(pValueStart),
				ValueSize:  uint64(payloadSpace - pValueStart),
				Record: Record{
					Timestamp: r.Timestamp,
					Tombstone: r.Tombstone,
					Key:       payload[:pValueStart],
					Value:     payload[pValueStart:payloadSpace],
				},
			}
			payload = payload[payloadSpace:]
			pValueStart = 0
		}
		buf := Encode(wr)
		_, err := s.CurrentBlock.Write(buf)
		if err != nil {
			return err
		}
		//fmt.Println("Partial record", buf, payload)

		// for loop seems unsafe, might have to refactor
		// to do
		for s.CurrentBlock.Remaining() < headerSize+len(payload) {
			//fmt.Println(s.CurrentBlock.Data)
			//fmt.Println(s.CurrentBlockIndex)
			err = s.MoveToNextBlock()
			if err != nil {
				return err
			}

			if s.CurrentBlock.Remaining() >= headerSize+len(payload) {
				break
			}

			payloadSpace = s.CurrentBlock.Remaining() - headerSize

			if payloadSpace < pValueStart { // part of key fits
				wr = WALRecord{
					FragType:   MIDDLE,
					RecType:    SINGLE, // to be updated
					TxnID:      0,      // to be updated
					SequenceID: sequenceID,
					KeySize:    uint64(payloadSpace),
					ValueSize:  0,
					Record: Record{
						Timestamp: r.Timestamp,
						Tombstone: r.Tombstone,
						Key:       payload[:payloadSpace],
						Value:     nil,
					},
				}
				payload = payload[payloadSpace:]
				pValueStart = pValueStart - payloadSpace
			} else if payloadSpace == pValueStart { // exactly key fits
				wr = WALRecord{
					FragType:   MIDDLE,
					RecType:    SINGLE, // to be updated
					TxnID:      0,      // to be updated
					SequenceID: sequenceID,
					KeySize:    uint64(pValueStart),
					ValueSize:  0,
					Record: Record{
						Timestamp: r.Timestamp,
						Tombstone: r.Tombstone,
						Key:       payload[:pValueStart],
						Value:     nil,
					},
				}
				payload = payload[pValueStart:]
				pValueStart = 0
			} else { // part of value fits
				wr = WALRecord{
					FragType:   MIDDLE,
					RecType:    SINGLE, // to be updated
					TxnID:      0,      // to be updated
					SequenceID: sequenceID,
					KeySize:    uint64(pValueStart),
					ValueSize:  uint64(payloadSpace - pValueStart),
					Record: Record{
						Timestamp: r.Timestamp,
						Tombstone: r.Tombstone,
						Key:       payload[:pValueStart],
						Value:     payload[pValueStart:payloadSpace],
					},
				}
				payload = payload[payloadSpace:]
				pValueStart = 0
			}
			buf := Encode(wr)
			_, err := s.CurrentBlock.Write(buf)
			if err != nil {
				return err
			}

		}
		//fmt.Println(payload)
		wr = WALRecord{
			FragType:   LAST,
			RecType:    SINGLE, // to be updated
			TxnID:      0,      // to be updated
			SequenceID: sequenceID,
			KeySize:    uint64(pValueStart),
			ValueSize:  uint64(len(payload) - pValueStart),
			Record: Record{
				Timestamp: r.Timestamp,
				Tombstone: r.Tombstone,
				Key:       payload[:pValueStart],
				Value:     payload[pValueStart:],
			},
		}
		buf = Encode(wr)
		_, err = s.CurrentBlock.Write(buf)
		if err != nil {
			return err
		}

		return nil

		// reminder for myself, when fragmenting the record, dont crc the encoded payload, crc each part separately
	} else if s.CurrentBlock.Remaining() <= headerSize { // not a single byte of payload can fit, pad the block
		err := s.MoveToNextBlock()
		if err != nil {
			return err
		}

		return s.Append(r, sequenceID) // crash if headersize > block size, but that should be checked way earlier
	}

	return fmt.Errorf("block remaining size error")
}

func (s *Segment) ReadAllRecords() ([]WALRecord, error) { //doesnt join fragments
	if s == nil {
		return nil, fmt.Errorf("segment is nil")
	}
	all := make([]WALRecord, 0)

	for i := 0; i < s.MaxBlocks; i++ {
		blockData, err := s.ReadBlock(uint32(i))
		if err != nil {
			return nil, err
		}

		recs, err := ReadBlockRecords(blockData)
		if err != nil {
			return nil, err
		}
		if len(recs) == 0 {
			break
		}
		all = append(all, recs...)
	}
	return all, nil
}

func ReadBlockRecords(data []byte) ([]WALRecord, error) {
	records := make([]WALRecord, 0)
	offset := 0
	for {
		if offset+KEY_START > len(data) {
			return records, nil
		}
		recSize, err := RecordSizeAt(data, offset)
		if err != nil {
			return records, nil
		}

		recBytes := data[offset : offset+recSize]
		rec, err := Decode(recBytes)
		if err != nil {
			return records, nil
		}

		records = append(records, rec)
		offset += recSize

		if offset == len(data) {
			return records, nil
		}
	}
}

func (s *Segment) ReadBlock(index uint32) ([]byte, error) {
	if index >= uint32(s.MaxBlocks) {
		return nil, fmt.Errorf("block index out of range")
	}
	bk := block.BlockKey{
		FilePath: s.Path,
		Offset:   index,
	}
	buff, err := s.BM.ReadNoCache(bk)
	if err != nil {
		return nil, err
	}
	return buff, nil
}

func (s *Segment) FlushCurrentBlock() error {
	if s.CurrentBlock == nil {
		return fmt.Errorf("current block is nil")
	}
	bk := block.BlockKey{
		FilePath: s.Path,
		Offset:   s.CurrentBlockIndex,
	}
	err := s.BM.WriteNoCache(bk, s.CurrentBlock.Data)
	if err != nil {
		return err
	}
	return nil
}

func (s *Segment) MoveToNextBlock() error {
	if s.CurrentBlockIndex+1 >= uint32(s.MaxBlocks) {
		return fmt.Errorf("segment is full")
	}
	err := s.FlushCurrentBlock()
	if err != nil {
		return err
	}
	s.CurrentBlockIndex++
	s.CurrentBlock = NewBlock(s.BlockSize)
	return nil
}

func (s *Segment) Sync() error {
	err := s.FlushCurrentBlock()
	if err != nil {
		return err
	}
	return s.BM.SyncFile(s.Path)
}

func (s *Segment) IsFull() bool {
	if s.CurrentBlock == nil {
		return true
	}
	minFragmentSize := KEY_START + 1
	return s.CurrentBlockIndex == uint32(s.MaxBlocks-1) && s.CurrentBlock.Remaining() < minFragmentSize
}

func RecordSizeAt(data []byte, offset int) (int, error) {
	remainingSize := len(data) - offset
	if remainingSize <= KEY_START {
		return 0, fmt.Errorf("Not enough space for record")
	}
	keySize := binary.LittleEndian.Uint64(data[offset+KEY_SIZE_START : offset+VALUE_SIZE_START])
	valueSize := binary.LittleEndian.Uint64(data[offset+VALUE_SIZE_START : offset+KEY_START])
	total := KEY_START + int(keySize) + int(valueSize)
	if offset+total > len(data) {
		return 0, fmt.Errorf("record exceedes block bounds")
	}
	return total, nil
}

func ScanBlock(data []byte) (int, bool, error) {
	offset := 0
	foundAny := false

	for {
		if offset+KEY_START > len(data) {
			return offset, foundAny, nil
		}

		recSize, err := RecordSizeAt(data, offset)
		if err != nil {
			return offset, foundAny, nil
		}

		recBytes := data[offset : offset+recSize]
		_, err = Decode(recBytes)
		if err != nil {
			return offset, foundAny, nil
		}
		foundAny = true
		offset += recSize

		if offset == len(data) {
			return offset, foundAny, nil
		}
	}
}

// Helper function, should be moved to block manager?
func FileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
