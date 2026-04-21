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
		info = BackupInfo{
			Id:            id,
			BaseId:        (*baseBackup).GetId(),
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

func (i IncrementalBackup) Backup(manifest sstable.Manifest) error {
	err := block.EnsureDir(i.info.SaveDirectory)
	if err != nil {
		return err
	}

	var fileNames []string
	var unsavedFiles []string
	for _, layer := range manifest.Layers {
		for _, sstableManifest := range layer {
			file := sstableManifest.BaseFileName
			fileNames = append(fileNames, filepath.Base(file))
			newFile := path.Join(i.info.SaveDirectory, filepath.Base(file))

			if i.ContainsFile(filepath.Base(file)) {
				continue
			}
			err := block.CopyFile(file, newFile)
			if err != nil {
				return err
			}
			unsavedFiles = append(unsavedFiles, filepath.Base(file))
		}
	}

	i.info.Files = fileNames
	i.info.NewFiles = unsavedFiles

	err = i.info.SaveToFile(path.Join(i.info.SaveDirectory, InfoFileName))
	if err != nil {
		return err
	}

	err = manifest.SaveTo(i.info.SaveDirectory)
	return nil
}

func (i IncrementalBackup) Restore(directory string) error {
	//TODO implement me
	panic("implement me")
}

func (i IncrementalBackup) GetInfo() *BackupInfo {
	return &i.info
}

func (i IncrementalBackup) GetId() string {
	return i.info.Id
}

func (i IncrementalBackup) ContainsFile(fileName string) bool {
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
