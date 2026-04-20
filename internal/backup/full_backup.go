package backup

import (
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
	info          BackupInfo
	saveDirectory string
}

// leave saveDirectory as nil if directory doesnt exist yet
func NewFullBackup(saveDirectory *string) *FullBackup {
	settings := config.GetSettings()
	var info BackupInfo

	if saveDirectory == nil {
		timestamp := time.Now().Unix()
		info = BackupInfo{
			Id:        strconv.FormatInt(timestamp, 10),
			BaseId:    "",
			Type:      enums.FullBackup,
			Timestamp: timestamp,
			Files:     nil,
		}
		savePath := path.Join(settings.SavePath, settings.Backup.SaveDirectory, info.Id)
		saveDirectory = &savePath
	} else {
		info = BackupInfo{}
		info.LoadFromFile(*saveDirectory)
	}

	return &FullBackup{
		info:          info,
		saveDirectory: *saveDirectory,
	}
}

// steps:
// 1. copy all sstables to saveDirectory
// 2. save backupMetadata
// 3. save manifest for restore
func (f FullBackup) Backup(manifest sstable.Manifest) error {
	err := block.EnsureDir(f.saveDirectory)
	if err != nil {
		return err
	}

	var files []string
	var newFileNames []string
	for _, layer := range manifest.Layers {
		for _, sstableManifest := range layer {
			file := sstableManifest.BaseFileName
			files = append(files, file)
			newFile := path.Join(f.saveDirectory, filepath.Base(file))
			newFileNames = append(newFileNames, newFile)
			err := block.CopyFile(file, newFile)
			if err != nil {
				return err
			}
		}
	}

	f.info.Files = newFileNames

	err = f.info.SaveToFile(path.Join(f.saveDirectory, InfoFileName))
	if err != nil {
		return err
	}

	err = manifest.SaveTo(f.saveDirectory)
	return err
}

func (f FullBackup) Restore() error {
	//TODO
	return nil
}

func (f FullBackup) GetInfo() error {
	//TODO implement me
	panic("implement me")
}
