package sstable

import (
	"fmt"
	"io"
	"os"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
)

// SegmentStorage abstracts physical storage of SSTable segments.
// It hides whether segments are stored in a single file or multiple files.
type SegmentStorage interface {
	WriteSegment(segType config.SegmentType, data []byte) (offset uint64, size uint32, err error)
	ReadSegment(segType config.SegmentType, offset uint64, size uint32) ([]byte, error)
	Close() error
	Sync() error
}

// SingleFileStorage STORES ALL SSTABLE SEGMENTS IN A SINGLE PHYSICAL FILE
type SingleFileStorage struct {
	file         *os.File            // single file all segments are written into
	offset       uint64              // current write offset in the file
	blockManager *block.BlockManager // block manager for writing blocks
}

func NewSingleFileStorage(filePath string, blockManager *block.BlockManager) (*SingleFileStorage, error) {
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, err
	}
	return &SingleFileStorage{file: file, offset: 0, blockManager: blockManager}, nil
}

func OpenSingleFileStorage(filePath string) (*SingleFileStorage, error) {
	file, err := os.OpenFile(filePath, os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}
	return &SingleFileStorage{file: file, offset: 0}, nil
}

// WriteSegment APPENDS SEGMENT TO A FILE AND RETURNS ITS OFFSET + SIZE
func (s *SingleFileStorage) WriteSegment(segType config.SegmentType, data []byte) (uint64, uint32, error) {
	if (segType == config.SegmentData || segType == config.SegmentIndex) && s.blockManager != nil {
		blockSize := uint64(s.blockManager.BlockSize())
		remainder := s.offset % blockSize
		if remainder != 0 {
			padding := make([]byte, blockSize-remainder)
			if _, err := s.file.Seek(int64(s.offset), io.SeekStart); err != nil {
				return 0, 0, err
			}
			if _, err := s.file.Write(padding); err != nil {
				return 0, 0, err
			}
			s.offset += uint64(len(padding))
		}
		if len(data) < s.blockManager.BlockSize() {
			pad := make([]byte, s.blockManager.BlockSize()-len(data))
			data = append(data, pad...)
		}
		offset := s.offset
		blockKey := block.BlockKey{
			FilePath: s.file.Name(),
			Offset:   uint32(s.offset / blockSize),
		}
		if err := s.blockManager.Write(blockKey, data); err != nil {
			return 0, 0, err
		}
		s.offset += uint64(len(data))
		if _, err := s.file.Seek(int64(s.offset), io.SeekStart); err != nil {
			return 0, 0, err
		}
		return offset, uint32(len(data)), nil
	}
	if _, err := s.file.Seek(int64(s.offset), io.SeekStart); err != nil {
		return 0, 0, err
	}
	offset := s.offset
	n, err := s.file.Write(data)
	if err != nil {
		return 0, 0, err
	}
	s.offset += uint64(n)
	return offset, uint32(n), nil
}

// ReadSegment READS SEGMENT FROM A FILE (OFFSET AND SIZE ARE FORWARDED FROM FOOTER)
func (s *SingleFileStorage) ReadSegment(segType config.SegmentType, offset uint64, size uint32) ([]byte, error) {
	if (segType == config.SegmentData || segType == config.SegmentIndex) && s.blockManager != nil {
		blockSize := uint64(s.blockManager.BlockSize())
		blockKey := block.BlockKey{
			FilePath: s.file.Name(),
			Offset:   uint32(offset / blockSize),
		}
		return s.blockManager.Read(blockKey)
	}
	data := make([]byte, size)
	nTotal := 0
	if _, err := s.file.Seek(int64(offset), io.SeekStart); err != nil {
		return nil, err
	}
	for nTotal < int(size) {
		n, err := s.file.Read(data[nTotal:])
		nTotal += n
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	if nTotal != int(size) {
		return nil, fmt.Errorf("expected to read %d bytes, got %d", size, nTotal)
	}
	return data, nil
}

func (s *SingleFileStorage) Close() error {
	if s.file != nil {
		return s.file.Close()
	}
	return nil
}

func (s *SingleFileStorage) Sync() error {
	if s.file != nil {
		return s.file.Sync()
	}
	return nil
}

func (s *SingleFileStorage) File() *os.File {
	return s.file
}

// MultiFileStorage STORES EACH SSTABLE SEGMENT IN A SEPARATE FILE
type MultiFileStorage struct {
	basePath     string                          // base path on which prefixes like .footer are added
	files        map[config.SegmentType]*os.File // open file handles per segment type
	offsets      map[config.SegmentType]uint64   // current write offsets per segment
	blockManager *block.BlockManager             // block manager for writing blocks
}

func NewMultiFileStorage(basePath string, blockManager *block.BlockManager) (*MultiFileStorage, error) {
	return &MultiFileStorage{
		basePath:     basePath,
		files:        make(map[config.SegmentType]*os.File),
		offsets:      make(map[config.SegmentType]uint64),
		blockManager: blockManager,
	}, nil
}

func OpenMultiFileStorage(basePath string) (*MultiFileStorage, error) {
	return &MultiFileStorage{
		basePath: basePath,
		files:    make(map[config.SegmentType]*os.File),
		offsets:  make(map[config.SegmentType]uint64),
	}, nil
}

// getOrCreateFile RETURNS AN OPEN FILE HANDLE FOR THE GIVEN SEGMENT TYPE
func (m *MultiFileStorage) getOrCreateFile(segType config.SegmentType) (*os.File, error) {
	if file, exists := m.files[segType]; exists {
		return file, nil
	}
	filePath := m.getFilePath(segType)
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return nil, err
	}
	m.files[segType] = file
	m.offsets[segType] = 0
	return file, nil
}

// getOrOpenFile RETURNS AN OPEN FILE HANDLE FOR THE GIVEN SEGMENT TYPE IN A READ ONLY MODE -> USED FOR OPENING EXISTING SSTABLES
func (m *MultiFileStorage) getOrOpenFile(segType config.SegmentType) (*os.File, error) {
	if file, exists := m.files[segType]; exists {
		return file, nil
	}
	filePath := m.getFilePath(segType)
	file, err := os.OpenFile(filePath, os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}
	m.files[segType] = file
	return file, nil
}

func (m *MultiFileStorage) getFilePath(segType config.SegmentType) string {
	switch segType {
	case config.SegmentData:
		return m.basePath + ".data"
	case config.SegmentFilter:
		return m.basePath + ".filter"
	case config.SegmentIndex:
		return m.basePath + ".index"
	case config.SegmentSummary:
		return m.basePath + ".summary"
	case config.SegmentMetadata:
		return m.basePath + ".metadata"
	case config.SegmentFooter:
		return m.basePath + ".footer"
	default:
		return m.basePath
	}
}

// WriteSegment APPENDS SEGMENT TO A FILE CORRESPONDING TO A SEGMENT TYPE AND RETURNS ITS OFFSET + SIZE WITHIN THE GIVEN FILE
func (m *MultiFileStorage) WriteSegment(segType config.SegmentType, data []byte) (uint64, uint32, error) {
	file, err := m.getOrCreateFile(segType)
	if err != nil {
		return 0, 0, err
	}
	offset := m.offsets[segType]
	if (segType == config.SegmentData || segType == config.SegmentIndex) && m.blockManager != nil {
		if len(data) < m.blockManager.BlockSize() {
			padding := make([]byte, m.blockManager.BlockSize()-len(data))
			data = append(data, padding...)
		}
		blockKey := block.BlockKey{
			FilePath: file.Name(),
			Offset:   uint32(offset / uint64(m.blockManager.BlockSize())),
		}
		if err := m.blockManager.Write(blockKey, data); err != nil {
			return 0, 0, err
		}

		m.offsets[segType] += uint64(len(data))
		return offset, uint32(len(data)), nil
	}
	n, err := file.Write(data)
	if err != nil {
		return 0, 0, err
	}
	m.offsets[segType] += uint64(n)
	return offset, uint32(n), nil
}

// ReadSegment READS SEGMENT FROM A FILE CORRESPONDING TO SEGMENT TYPE (OFFSET AND SIZE ARE FORWARDED FROM FOOTER)
func (m *MultiFileStorage) ReadSegment(segType config.SegmentType, offset uint64, size uint32) ([]byte, error) {
	if (segType == config.SegmentData || segType == config.SegmentIndex) && m.blockManager != nil {
		file, err := m.getOrOpenFile(segType)
		if err != nil {
			return nil, err
		}
		blockSize := uint64(m.blockManager.BlockSize())
		blockKey := block.BlockKey{
			FilePath: file.Name(),
			Offset:   uint32(offset / blockSize),
		}
		return m.blockManager.Read(blockKey)
	}
	file, err := m.getOrOpenFile(segType)
	if err != nil {
		return nil, err
	}
	data := make([]byte, size)
	nTotal := 0
	if _, err = file.Seek(int64(offset), io.SeekStart); err != nil {
		return nil, err
	}
	for nTotal < int(size) {
		n, err := file.Read(data[nTotal:])
		nTotal += n
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	if nTotal != int(size) {
		return nil, fmt.Errorf("expected to read %d bytes, got %d", size, nTotal)
	}
	return data, nil
}

func (m *MultiFileStorage) Close() error {
	var lastErr error
	for _, file := range m.files {
		if err := file.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (m *MultiFileStorage) Sync() error {
	var lastErr error
	for _, file := range m.files {
		if err := file.Sync(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// CreateStorage CREATES SINGLE FILE / MULTI FILE STORAGE BASED ON SSTABLE FORMAT CONFIGURATION
func CreateStorage(basePath string, config *config.Config, blockManager *block.BlockManager) (SegmentStorage, error) {
	if config.SSTable.Format == 0 {
		return NewSingleFileStorage(basePath, blockManager)
	}
	return NewMultiFileStorage(basePath, blockManager)
}

// OpenStorage OPENS SINGLE FILE / MULTI FILE STORAGE BASED ON SSTABLE FORMAT CONFIGURATION
func OpenStorage(basePath string, config *config.Config) (SegmentStorage, error) {
	if config.SSTable.Format == 0 {
		return OpenSingleFileStorage(basePath)
	}
	return OpenMultiFileStorage(basePath)
}

func (s *SingleFileStorage) Size() (uint64, error) {
	info, err := s.file.Stat()
	if err != nil {
		return 0, err
	}
	return uint64(info.Size()), nil
}
