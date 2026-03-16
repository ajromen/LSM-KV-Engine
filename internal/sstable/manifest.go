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
	Id           int                 `json:"id"`
	BaseFileName string              `json:"base_file_name"`
	Format       enums.SSTableFormat `json:"is_multi"`
	Layer        uint64              `json:"layer"`
}

type Manifest struct {
	FileDir       string                    `json:"file_dir"`
	NextSStableId int                       `json:"next_stable_id"`
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
				sstableFiles[basePath] = enums.FormatMultiFile
			}
		}
	}
	sortedFiles := make([]string, 0, len(sstableFiles))
	for file := range sstableFiles {
		sortedFiles = append(sortedFiles, file)
	}
	sort.Strings(sortedFiles)
	for _, filePath := range sortedFiles {
		var id int
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

func (m *Manifest) Save() error {
	err := block.WriteJSON(path.Join(m.FileDir, ManifestFileName), m)
	if err != nil {
		return err
	}
	return nil
}

func (m *Manifest) MoveSSTable(id, fromLayer, toLayer int) error {
	layer := m.Layers[fromLayer]
	var sstable SSTableManifest
	for i, sst := range layer {
		if sst.Id == id {
			sstable = sst
			m.Layers[fromLayer] = append(layer[:i], layer[i+1:]...)
			break
		}
	}
	m.Layers[toLayer] = append(m.Layers[toLayer], sstable)
	return m.Save()
}

func (m *Manifest) AddSSTable(sstManifest SSTableManifest) error {
	layer := int(sstManifest.Layer)
	m.Layers[layer] = append(m.Layers[layer], sstManifest)
	err := m.Save()
	if err != nil {
		return err
	}
	return nil
}

func (m *Manifest) RemoveSSTable(id int, layerNumber int) error {
	layers := m.Layers[layerNumber]
	for i, sst := range layers {
		if sst.Id == id {
			m.Layers[layerNumber] = append(layers[:i], layers[i+1:]...)
			return m.Save()
		}
	}
	return fmt.Errorf("sstable %d not found in layer %d", id, layerNumber)
}

func (m *Manifest) IncrementId() {
	m.NextSStableId++
	if m.NextSStableId >= 10_000_000-1 {
		m.NextSStableId = 0
	}
}
