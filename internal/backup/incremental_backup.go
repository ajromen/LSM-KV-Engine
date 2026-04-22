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

type IncrementalBackup struct {
	info BackupInfo
}

func NewIncrementalBackup(saveDirectory *string, baseBackup *IBackup) *IncrementalBackup {
	settings := config.GetSettings()
	var info BackupInfo

	if saveDirectory == nil {
		timestamp := time.Now().Unix()
		id := strconv.FormatInt(timestamp, 10)
		savePath := path.Join(settings.SavePath, settings.Backup.SaveDirectory, id)
		saveDirectory = &savePath

		baseId := ""
		if baseBackup != nil {
			baseId = (*baseBackup).GetId()
		}

		info = BackupInfo{
			Id:            id,
			BaseId:        baseId,
			Base:          baseBackup,
			Type:          enums.IncrementalBackup,
			Timestamp:     timestamp,
			Files:         nil,
			NewFiles:      nil,
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
	return &IncrementalBackup{
		info: info,
	}
}

func (i *IncrementalBackup) Backup(manifest sstable.Manifest) error {
	err := block.EnsureDir(i.info.SaveDirectory)
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
	var unsavedFiles []string
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

		fileNames = append(fileNames, name)
		if i.ContainsFile(name) {
			continue
		}

		src := filepath.Join(dir, name)
		dst := filepath.Join(i.info.SaveDirectory, name)
		if err := block.CopyFile(src, dst); err != nil {
			return fmt.Errorf("failed to copy %s: %w", name, err)
		}
		unsavedFiles = append(unsavedFiles, name)
	}

	i.info.Files = fileNames
	i.info.NewFiles = unsavedFiles
	if err := i.info.SaveToFile(filepath.Join(i.info.SaveDirectory, InfoFileName)); err != nil {
		return err
	}
	return manifest.SaveTo(i.info.SaveDirectory)
}

func (i *IncrementalBackup) Restore(directory string) error {
	if i.info.Base == nil {
		return fmt.Errorf("incremental backup %s has no base", i.info.Id)
	}
	err := (*i.info.Base).Restore(directory)
	if err != nil {
		return fmt.Errorf("failed to restore base backup: %w", err)
	}

	for _, file := range i.info.NewFiles {
		filePath := path.Join(i.info.SaveDirectory, file)
		destPath := path.Join(directory, filepath.Base(file))
		err := block.CopyFile(filePath, destPath)
		if err != nil {
			return err
		}
	}

	destManifest := path.Join(directory, sstable.ManifestFileName)
	err = block.CopyFile(path.Join(i.info.SaveDirectory, sstable.ManifestFileName), destManifest)
	if err != nil {
		return fmt.Errorf("failed to restore manifest: %w", err)
	}

	return nil
}

func (i *IncrementalBackup) GetInfo() *BackupInfo {
	return &i.info
}

func (i *IncrementalBackup) GetId() string {
	return i.info.Id
}

func (i *IncrementalBackup) ContainsFile(fileName string) bool {
	for _, file := range i.info.NewFiles {
		if file == fileName {
			return true
		}
	}

	if i.info.Base == nil {
		return false
	}

	// check if some lover level contains file
	return (*i.info.Base).ContainsFile(fileName)
}
