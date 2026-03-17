package sstable

import (
	"os"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
)

// SegmentStorage abstracts physical storage of SSTable segments.
// It hides whether segments are stored in a single file or multiple files.
type SegmentStorage interface {
	WriteSegment(segType enums.SegmentType, data []byte) (offset uint64, size uint32, err error)
	ReadSegment(segType enums.SegmentType, offset uint64, size uint32) ([]byte, error)
	Delete()
	Close() error
	Sync() error
	SetBlockManager(bm *block.BlockManager)
}

// SingleFileStorage STORES ALL SSTABLE SEGMENTS IN A SINGLE PHYSICAL FILE
type SingleFileStorage struct {
	file         *os.File            // single file all segments are written into
	offset       uint64              // current write offset in the file
	BlockManager *block.BlockManager // block manager for writing blocks
}

func (s *SingleFileStorage) Delete() {
	s.Close()
	os.Remove(s.file.Name())
}

func (s *SingleFileStorage) SetBlockManager(bm *block.BlockManager) {
	s.BlockManager = bm
}

func (m *MultiFileStorage) SetBlockManager(bm *block.BlockManager) {
	m.BlockManager = bm
}

func NewSingleFileStorage(filePath string, blockManager *block.BlockManager) (*SingleFileStorage, error) {
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, err
	}
	return &SingleFileStorage{file: file, offset: 0, BlockManager: blockManager}, nil
}

func OpenSingleFileStorage(filePath string) (*SingleFileStorage, error) {
	file, err := os.OpenFile(filePath, os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}
	return &SingleFileStorage{file: file, offset: 0}, nil
}

// WriteSegment APPENDS SEGMENT TO A FILE AND RETURNS ITS OFFSET + SIZE
func (s *SingleFileStorage) WriteSegment(segType enums.SegmentType, data []byte) (uint64, uint32, error) {
	if (segType == enums.SegmentData || segType == enums.SegmentIndex) && s.BlockManager != nil {
		blockSize := uint64(s.BlockManager.BlockSize())
		remainder := s.offset % blockSize
		if remainder != 0 {
			padding := make([]byte, blockSize-remainder)

			err := block.WriteNoBlock(s.file, s.offset, padding)
			if err != nil {
				return 0, 0, err
			}

			s.offset += uint64(len(padding))
		}
		if len(data) < s.BlockManager.BlockSize() {
			pad := make([]byte, s.BlockManager.BlockSize()-len(data))
			data = append(data, pad...)
		}
		offset := s.offset
		blockKey := block.BlockKey{
			FilePath: s.file.Name(),
			Offset:   uint32(s.offset / blockSize),
		}
		if err := s.BlockManager.Write(blockKey, data); err != nil {
			return 0, 0, err
		}
		s.offset += uint64(len(data))
		return offset, uint32(len(data)), nil
	}
	offset := s.offset
	err := block.WriteNoBlock(s.file, offset, data)
	if err != nil {
		return 0, 0, err
	}
	s.offset += uint64(len(data))
	return offset, uint32(len(data)), nil
}

// ReadSegment READS SEGMENT FROM A FILE (OFFSET AND SIZE ARE FORWARDED FROM FOOTER)
func (s *SingleFileStorage) ReadSegment(segType enums.SegmentType, offset uint64, size uint32) ([]byte, error) {
	if (segType == enums.SegmentData || segType == enums.SegmentIndex) && s.BlockManager != nil {
		blockSize := uint64(s.BlockManager.BlockSize())
		blockKey := block.BlockKey{
			FilePath: s.file.Name(),
			Offset:   uint32(offset / blockSize),
		}
		return s.BlockManager.Read(blockKey)
	}
	data, err := block.ReadNoBlock(s.file, offset, size)
	if err != nil {
		return nil, err
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
	basePath     string                         // base path on which prefixes like .footer are added
	files        map[enums.SegmentType]*os.File // open file handles per segment type
	offsets      map[enums.SegmentType]uint64   // current write offsets per segment
	BlockManager *block.BlockManager            // block manager for writing blocks
}

func (m *MultiFileStorage) Delete() {
	for _, file := range m.files {
		file.Close()
		os.Remove(file.Name())
	}
}

func NewMultiFileStorage(basePath string, blockManager *block.BlockManager) (*MultiFileStorage, error) {
	return &MultiFileStorage{
		basePath:     basePath,
		files:        make(map[enums.SegmentType]*os.File),
		offsets:      make(map[enums.SegmentType]uint64),
		BlockManager: blockManager,
	}, nil
}

func OpenMultiFileStorage(basePath string) (*MultiFileStorage, error) {
	return &MultiFileStorage{
		basePath: basePath,
		files:    make(map[enums.SegmentType]*os.File),
		offsets:  make(map[enums.SegmentType]uint64),
	}, nil
}

// getOrCreateFile RETURNS AN OPEN FILE HANDLE FOR THE GIVEN SEGMENT TYPE
func (m *MultiFileStorage) getOrCreateFile(segType enums.SegmentType) (*os.File, error) {
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
func (m *MultiFileStorage) getOrOpenFile(segType enums.SegmentType) (*os.File, error) {
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

func (m *MultiFileStorage) getFilePath(segType enums.SegmentType) string {
	switch segType {
	case enums.SegmentData:
		return m.basePath + ".data"
	case enums.SegmentFilter:
		return m.basePath + ".filter"
	case enums.SegmentIndex:
		return m.basePath + ".index"
	case enums.SegmentSummary:
		return m.basePath + ".summary"
	case enums.SegmentMetadata:
		return m.basePath + ".metadata"
	case enums.SegmentFooter:
		return m.basePath + ".footer"
	case config.SegmentDictionary:
		return m.basePath + ".dictionary"
	default:
		return m.basePath
	}
}

// WriteSegment APPENDS SEGMENT TO A FILE CORRESPONDING TO A SEGMENT TYPE AND RETURNS ITS OFFSET + SIZE WITHIN THE GIVEN FILE
func (m *MultiFileStorage) WriteSegment(segType enums.SegmentType, data []byte) (uint64, uint32, error) {
	file, err := m.getOrCreateFile(segType)
	if err != nil {
		return 0, 0, err
	}
	offset := m.offsets[segType]
	if (segType == enums.SegmentData || segType == enums.SegmentIndex) && m.BlockManager != nil {
		if len(data) < m.BlockManager.BlockSize() {
			padding := make([]byte, m.BlockManager.BlockSize()-len(data))
			data = append(data, padding...)
		}
		blockKey := block.BlockKey{
			FilePath: file.Name(),
			Offset:   uint32(offset / uint64(m.BlockManager.BlockSize())),
		}
		if err := m.BlockManager.Write(blockKey, data); err != nil {
			return 0, 0, err
		}

		m.offsets[segType] += uint64(len(data))
		return offset, uint32(len(data)), nil
	}
	err = block.WriteNoBlock(file, offset, data)
	if err != nil {
		return 0, 0, err
	}

	m.offsets[segType] += uint64(len(data))
	return offset, uint32(len(data)), nil
}

// ReadSegment READS SEGMENT FROM A FILE CORRESPONDING TO SEGMENT TYPE (OFFSET AND SIZE ARE FORWARDED FROM FOOTER)
func (m *MultiFileStorage) ReadSegment(segType enums.SegmentType, offset uint64, size uint32) ([]byte, error) {
	file, err := m.getOrOpenFile(segType)
	if err != nil {
		return nil, err
	}
	if (segType == enums.SegmentData || segType == enums.SegmentIndex) && m.BlockManager != nil {
		blockSize := uint64(m.BlockManager.BlockSize())
		blockKey := block.BlockKey{
			FilePath: file.Name(),
			Offset:   uint32(offset / blockSize),
		}
		return m.BlockManager.Read(blockKey)
	}

	data, err := block.ReadNoBlock(file, offset, size)
	if err != nil {
		return nil, err
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
