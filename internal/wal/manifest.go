package wal

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
)

const ManifestFileName = "WAL_MANIFEST.json"

type WALManifestEntry struct {
	SegmentID      uint64 `json:"segment_id"`
	ValidFromBlock uint64 `json:"valid_from_block"`
}

type WALManifest struct {
	FileDir      string             `json:"file_dir"`
	LowWatermark uint64             `json:"low_watermark"`
	Segments     []WALManifestEntry `json:"segments"`
}

func NewWALManifest(fileDir string) (*WALManifest, error) {
	err := block.EnsureDir(fileDir)
	if err != nil {
		return nil, err
	}

	manifest := &WALManifest{
		FileDir:      fileDir,
		LowWatermark: 0,
		Segments:     make([]WALManifestEntry, 0),
	}

	_, err = os.Stat(manifest.filePath())
	if err != nil {
		err = manifest.reconstruct(fileDir)
		if err != nil {
			return nil, err
		}
		if config.GetSettings().Debug {
			fmt.Printf("Reconstructed WAL manifest from folder\n")
		}
	} else {
		err = manifest.load()
		if err != nil {
			return nil, err
		}
		if config.GetSettings().Debug {
			fmt.Printf("WAL manifest loaded from file\n")
		}
	}

	return manifest, nil
}

func (m *WALManifest) reconstruct(fileDir string) error {
	entries, err := os.ReadDir(fileDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if len(name) == 0 {
			continue
		}
		id, err := ParseSegmentID(name)
		if err != nil {
			continue
		}
		m.Segments = append(m.Segments, WALManifestEntry{SegmentID: id})
	}

	return m.Save()
}

func (m *WALManifest) filePath() string {
	return filepath.Join(m.FileDir, ManifestFileName)
}

func (m *WALManifest) Save() error {
	return block.WriteJSON(m.filePath(), m)
}

func (m *WALManifest) load() error {
	return block.ReadJSON(m.filePath(), m)
}

func (m *WALManifest) AddSegment(id uint64) error {
	m.Segments = append(m.Segments, WALManifestEntry{SegmentID: id})
	return m.Save()
}

func (m *WALManifest) RemoveSegment(id uint64) error {
	for i, seg := range m.Segments {
		if seg.SegmentID == id {
			m.Segments = append(m.Segments[:i], m.Segments[i+1:]...)
			break
		}
	}
	return m.Save()
}

func (m *WALManifest) SetLowWatermark(segmentID uint64) error {
	if segmentID < m.LowWatermark {
		return fmt.Errorf("low watermark cannot move backwards")
	}
	m.LowWatermark = segmentID
	return m.Save()
}
