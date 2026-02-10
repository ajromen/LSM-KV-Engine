package sstable

import (
	"fmt"
	"io"
	"os"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
)

type SegmentStorage interface {
	WriteSegment(segType config.SegmentType, data []byte) (offset uint64, size uint32, err error)
	ReadSegment(segType config.SegmentType, offset uint64, size uint32) ([]byte, error)
	Close() error
	Sync() error
}

type SingleFileStorage struct {
	file   *os.File
	offset uint64
}

func NewSingleFileStorage(filePath string) (*SingleFileStorage, error) {
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, err
	}
	return &SingleFileStorage{file: file, offset: 0}, nil
}

func OpenSingleFileStorage(filePath string) (*SingleFileStorage, error) {
	file, err := os.OpenFile(filePath, os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}
	return &SingleFileStorage{file: file, offset: 0}, nil
}

func (s *SingleFileStorage) WriteSegment(segType config.SegmentType, data []byte) (uint64, uint32, error) {
	n, err := s.file.Write(data)
	if err != nil {
		return 0, 0, err
	}
	offset := s.offset
	s.offset += uint64(n)
	return offset, uint32(n), nil
}

func (s *SingleFileStorage) ReadSegment(segType config.SegmentType, offset uint64, size uint32) ([]byte, error) {
	data := make([]byte, size)
	_, err := s.file.Seek(int64(offset), io.SeekStart)
	if err != nil {
		return nil, err
	}
	n, err := s.file.Read(data)
	if err != nil {
		return nil, err
	}
	if n != int(size) {
		return nil, fmt.Errorf("expected to read %d bytes, got %d", size, n)
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

type MultiFileStorage struct {
	basePath string
	files    map[config.SegmentType]*os.File
	offsets  map[config.SegmentType]uint64
}

func NewMultiFileStorage(basePath string) (*MultiFileStorage, error) {
	return &MultiFileStorage{
		basePath: basePath,
		files:    make(map[config.SegmentType]*os.File),
		offsets:  make(map[config.SegmentType]uint64),
	}, nil
}

func OpenMultiFileStorage(basePath string) (*MultiFileStorage, error) {
	return &MultiFileStorage{
		basePath: basePath,
		files:    make(map[config.SegmentType]*os.File),
		offsets:  make(map[config.SegmentType]uint64),
	}, nil
}

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
		return m.basePath
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

func (m *MultiFileStorage) WriteSegment(segType config.SegmentType, data []byte) (uint64, uint32, error) {
	file, err := m.getOrCreateFile(segType)
	if err != nil {
		return 0, 0, err
	}
	n, err := file.Write(data)
	if err != nil {
		return 0, 0, err
	}
	offset := m.offsets[segType]
	m.offsets[segType] += uint64(n)
	return offset, uint32(n), nil
}

func (m *MultiFileStorage) ReadSegment(segType config.SegmentType, offset uint64, size uint32) ([]byte, error) {
	file, err := m.getOrOpenFile(segType)
	if err != nil {
		return nil, err
	}
	data := make([]byte, size)
	_, err = file.Seek(int64(offset), io.SeekStart)
	if err != nil {
		return nil, err
	}
	n, err := file.Read(data)
	if err != nil {
		return nil, err
	}
	if n != int(size) {
		return nil, fmt.Errorf("expected to read %d bytes, got %d", size, n)
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

func CreateStorage(basePath string, config config.SSTableConfig) (SegmentStorage, error) {
	if config.Format == 0 {
		return NewSingleFileStorage(basePath)
	}
	return NewMultiFileStorage(basePath)
}

func OpenStorage(basePath string, config config.SSTableConfig) (SegmentStorage, error) {
	if config.Format == 1 {
		return OpenSingleFileStorage(basePath)
	}
	return OpenMultiFileStorage(basePath)
}
