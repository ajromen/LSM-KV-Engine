package wal

import (
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
		// to do
		return nil, fmt.Errorf("not implemented yet")
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

func (s *Segment) Append(r Record) error {
	headerSize := KEY_START
	totalSize := headerSize + len(r.Key) + len(r.Value)
	if s.CurrentBlock.Remaining() >= totalSize { // if full record can fit
		wr := WALRecord{
			FragType:  FULL,
			RecType:   SINGLE, // to be updated
			TxnID:     0,      // to be updated
			KeySize:   uint64(len(r.Key)),
			ValueSize: uint64(len(r.Value)),
			Record:    r,
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
				FragType:  FIRST,
				RecType:   SINGLE, // to be updated
				TxnID:     0,      // to be updated
				KeySize:   uint64(payloadSpace),
				ValueSize: 0,
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
				FragType:  FIRST,
				RecType:   SINGLE, // to be updated
				TxnID:     0,      // to be updated
				KeySize:   uint64(pValueStart),
				ValueSize: 0,
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
				FragType:  FIRST,
				RecType:   SINGLE, // to be updated
				TxnID:     0,      // to be updated
				KeySize:   uint64(pValueStart),
				ValueSize: uint64(payloadSpace - pValueStart),
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
					FragType:  MIDDLE,
					RecType:   SINGLE, // to be updated
					TxnID:     0,      // to be updated
					KeySize:   uint64(payloadSpace),
					ValueSize: 0,
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
					FragType:  MIDDLE,
					RecType:   SINGLE, // to be updated
					TxnID:     0,      // to be updated
					KeySize:   uint64(pValueStart),
					ValueSize: 0,
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
					FragType:  MIDDLE,
					RecType:   SINGLE, // to be updated
					TxnID:     0,      // to be updated
					KeySize:   uint64(pValueStart),
					ValueSize: uint64(payloadSpace - pValueStart),
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
		fmt.Println(payload)
		wr = WALRecord{
			FragType:  LAST,
			RecType:   SINGLE, // to be updated
			TxnID:     0,      // to be updated
			KeySize:   uint64(pValueStart),
			ValueSize: uint64(len(payload) - pValueStart),
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

		return s.Append(r) // crash if headersize > block size, but that should be checked way earlier
	}

	return fmt.Errorf("block remaining size error")
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
