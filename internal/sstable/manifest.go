package sstable

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
)

const ManifestFileName = "MANIFEST.json"

type SSTableManifest struct {
	Id           uint64              `json:"id"`
	BaseFileName string              `json:"base_file_name"`
	Format       enums.SSTableFormat `json:"is_multi"`
	Layer        uint64              `json:"layer"`
}

type Manifest struct {
	FileDir       string                    `json:"file_dir"`
	NextSStableId uint64                    `json:"next_stable_id"`
	Layers        map[int][]SSTableManifest `json:"Layers"`
}

func NewManifest(fileDir string) (*Manifest, error) {
	manifest := &Manifest{
		FileDir: fileDir,
		Layers:  make(map[int][]SSTableManifest),
	}
	_, err := os.Stat(filepath.Join(fileDir, ManifestFileName))
	if err != nil {
		err = manifest.reconstruct(fileDir)
		if err != nil {
			return nil, err
		}
	} else {
		err := manifest.load()
		if err != nil {
			return nil, err
		}
	}
	return manifest, nil
}

func (m *Manifest) reconstruct(fileDir string) error {
	files, err := os.ReadDir(fileDir)
	if err != nil {
		return err
	}
	sstableFiles := make(map[string]enums.SSTableFormat)
	for _, file := range files {
		if !file.IsDir() {
			name := file.Name()
			// Single-file: 000000.sst
			if filepath.Ext(name) == ".sst" && !strings.Contains(strings.TrimSuffix(name, ".sst"), ".") {
				sstableFiles[filepath.Join(fileDir, name)] = enums.FormatSingleFile
			}
			// Multi-file: 000000.sst.data -> add 000000.sst to list
			if strings.HasSuffix(name, ".sst.data") {
				basePath := filepath.Join(fileDir, strings.TrimSuffix(name, ".data"))
				sstableFiles[basePath] = enums.FormatSingleFile
			}
		}
	}
	sortedFiles := make([]string, 0, len(sstableFiles))
	for file := range sstableFiles {
		sortedFiles = append(sortedFiles, file)
	}
	sort.Strings(sortedFiles)
	for _, filePath := range sortedFiles {
		var id uint64
		_, err := fmt.Sscanf(filepath.Base(filePath), "%d.sst", &id)
		if err != nil {
			return err
		}
		if id >= m.NextSStableId {
			m.NextSStableId = id + 1
		}
		sst := SSTableManifest{
			Layer:        0,
			BaseFileName: filepath.Base(filePath),
			Format:       sstableFiles[filePath],
			Id:           id,
		}
		m.Layers[0] = append(m.Layers[0], sst)
	}
	return nil
}

func (m *Manifest) load() error {
	err := block.ReadJSON(filepath.Join(m.FileDir, ManifestFileName), m)
	if err != nil {
		return err
	}
	return nil
}

func (m *Manifest) save() error {
	err := block.WriteJSON(path.Join(m.FileDir, ManifestFileName), m)
	if err != nil {
		return err
	}
	return nil
}

func (m *Manifest) AddSSTable(sstManifest SSTableManifest) error {
	layer := int(sstManifest.Layer)
	m.Layers[layer] = append(m.Layers[layer], sstManifest)
	err := m.save()
	if err != nil {
		return err
	}
	return nil
}
