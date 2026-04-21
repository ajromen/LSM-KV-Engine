package backup

import (
	"fmt"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
	"github.com/ajromen/LSM-KV-Engine/internal/sstable"
)

type FullBackup struct {
	info BackupInfo
}

// leave saveDirectory as nil if directory doesnt exist yet
func NewFullBackup(saveDirectory *string) *FullBackup {
	settings := config.GetSettings()
	var info BackupInfo

	if saveDirectory == nil {
		timestamp := time.Now().Unix()
		savePath := path.Join(settings.SavePath, settings.Backup.SaveDirectory, info.Id)
		saveDirectory = &savePath
		info = BackupInfo{
			Id:            strconv.FormatInt(timestamp, 10),
			BaseId:        "",
			Type:          enums.FullBackup,
			Timestamp:     timestamp,
			Files:         nil,
			SaveDirectory: *saveDirectory,
		}
	} else {
		info = BackupInfo{}
		err := info.LoadFromFile(*saveDirectory)
		info.SaveDirectory = *saveDirectory
		if err != nil {
			panic(err)
		}
	}

	return &FullBackup{
		info: info,
	}
}

// steps:
// 1. copy all sstables to saveDirectory
// 2. save backupMetadata
// 3. save manifest for restore
func (f FullBackup) Backup(manifest sstable.Manifest) error {
	err := block.EnsureDir(f.info.SaveDirectory)
	if err != nil {
		return err
	}

	var files []string
	var newFileNames []string
	for _, layer := range manifest.Layers {
		for _, sstableManifest := range layer {
			file := sstableManifest.BaseFileName
			files = append(files, file)
			newFile := path.Join(f.info.SaveDirectory, filepath.Base(file))
			newFileNames = append(newFileNames, newFile)
			err := block.CopyFile(file, newFile)
			if err != nil {
				return err
			}
		}
	}

	f.info.Files = newFileNames

	err = f.info.SaveToFile(path.Join(f.info.SaveDirectory, InfoFileName))
	if err != nil {
		return err
	}

	err = manifest.SaveTo(f.info.SaveDirectory)
	return err
}

func (f FullBackup) Restore(directory string) error {
	for _, file := range f.info.Files {
		err := block.CopyFile(file, directory)
		if err != nil {
			return err
		}
	}
	err := block.CopyFile(path.Join(f.info.SaveDirectory, sstable.ManifestFileName), directory)
	if err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}
	return nil
}

func (f FullBackup) GetInfo() *BackupInfo {
	return &f.info
}

func (f FullBackup) GetId() string {
	return f.info.Id
}
