package backup

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
	"github.com/ajromen/LSM-KV-Engine/internal/sstable"
)

type Checkpoint struct {
	info BackupInfo
}

func NewCheckpoint(saveDirectory *string) *Checkpoint {
	settings := config.GetSettings()
	var info BackupInfo

	if saveDirectory == nil {
		timestamp := time.Now().Unix()
		id := strconv.FormatInt(timestamp, 10)
		savePath := path.Join(settings.SavePath, settings.Backup.SaveDirectory, id)
		saveDirectory = &savePath
		info = BackupInfo{
			Id:            id,
			BaseId:        "",
			Type:          enums.Checkpoint,
			Timestamp:     timestamp,
			Files:         nil,
			SaveDirectory: *saveDirectory,
		}
	} else {
		info = BackupInfo{}
		err := info.LoadFromFile(path.Join(*saveDirectory, InfoFileName))
		info.SaveDirectory = *saveDirectory
		if err != nil {
			panic(err)
		}
	}

	return &Checkpoint{
		info: info,
	}
}

func (c *Checkpoint) Backup(manifest sstable.Manifest) error {
	err := block.EnsureDir(c.info.SaveDirectory)
	if err != nil {
		return err
	}

	baseNames := make(map[string]struct{})
	for _, layer := range manifest.Layers {
		for _, sst := range layer {
			baseNames[filepath.Base(sst.BaseFileName)] = struct{}{}
		}
	}

	dir := manifest.FileDir
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read data dir: %w", err)
	}

	var fileNames []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == sstable.ManifestFileName {
			continue
		}
		matched := false
		for base := range baseNames {
			if name == base || strings.HasPrefix(name, base+".") {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}

		src := filepath.Join(dir, name)
		dst := filepath.Join(c.info.SaveDirectory, name)
		if err := block.CreateHardLink(src, dst); err != nil {
			return fmt.Errorf("failed to hardlink %s: %w", name, err)
		}
		fileNames = append(fileNames, name)
	}

	c.info.Files = fileNames
	if err := c.info.SaveToFile(filepath.Join(c.info.SaveDirectory, InfoFileName)); err != nil {
		return err
	}
	return manifest.SaveTo(c.info.SaveDirectory)
}

func (c *Checkpoint) GetInfo() *BackupInfo {
	return &c.info
}

func (c *Checkpoint) GetId() string {
	return c.info.Id
}

func (c *Checkpoint) Restore(directory string) error {
	for _, file := range c.info.Files {
		filePath := path.Join(c.info.SaveDirectory, file)
		destPath := path.Join(directory, filepath.Base(file))
		err := block.CopyFile(filePath, destPath)
		if err != nil {
			return err
		}
	}
	destManifest := path.Join(directory, sstable.ManifestFileName)
	err := block.CopyFile(path.Join(c.info.SaveDirectory, sstable.ManifestFileName), destManifest)
	if err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}
	return nil
}

func (c *Checkpoint) ContainsFile(filename string) bool {
	for _, file := range c.info.Files {
		if file == filename {
			return true
		}
	}
	return false
}
