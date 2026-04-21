package backup

import (
	"path"
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

func NewIncrementalBackup(saveDirectory *string, baseId string) *IncrementalBackup {
	settings := config.GetSettings()
	var info BackupInfo

	if saveDirectory == nil {
		timestamp := time.Now().Unix()
		savePath := path.Join(settings.SavePath, settings.Backup.SaveDirectory, info.Id)
		saveDirectory = &savePath
		info = BackupInfo{
			Id:            strconv.FormatInt(timestamp, 10),
			BaseId:        baseId,
			Type:          enums.IncrementalBackup,
			Timestamp:     timestamp,
			Files:         nil,
			NewFiles:      nil,
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
	return &IncrementalBackup{
		info: info,
	}
}

func (i IncrementalBackup) Backup(manifest sstable.Manifest) error {
	err := block.EnsureDir(i.info.SaveDirectory)
	if err != nil {
		return err
	}

	//var files []string
	//var newFileNames []string
	//for _, layer := range manifest.Layers {
	//	for _, sstableManifest := range layer {
	//		file := sstableManifest.BaseFileName
	//		files = append(files, file)
	//		newFile := path.Join(f.info.SaveDirectory, filepath.Base(file))
	//		newFileNames = append(newFileNames, newFile)
	//		err := block.CopyFile(file, newFile)
	//		if err != nil {
	//			return err
	//		}
	//	}
	//}
	//
	//f.info.Files = newFileNames
	//
	//err = f.info.SaveToFile(path.Join(f.info.SaveDirectory, InfoFileName))
	//if err != nil {
	//	return err
	//}
	//
	//err = manifest.SaveTo(f.info.SaveDirectory)
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
