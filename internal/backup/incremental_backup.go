package backup

import (
	"path"
	"strconv"
	"time"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
	"github.com/ajromen/LSM-KV-Engine/internal/sstable"
)

type IncrementalBackup struct {
	info          BackupInfo
	SaveDirectory string
}

func NewIncrementalBackup(saveDirectory *string) *IncrementalBackup {
	settings := config.GetSettings()
	var info BackupInfo

	if saveDirectory == nil {
		timestamp := time.Now().Unix()
		info = BackupInfo{
			Id:        strconv.FormatInt(timestamp, 10),
			BaseId:    "",
			Type:      enums.IncrementalBackup,
			Timestamp: timestamp,
			Files:     nil,
		}
		savePath := path.Join(settings.SavePath, settings.Backup.SaveDirectory, info.Id+BackupFileExtension)
		saveDirectory = &savePath
	} else {
		info = BackupInfo{}
		err := info.LoadFromFile(*saveDirectory)
		if err != nil {
			panic(err)
		}
	}
	return &IncrementalBackup{
		info:          info,
		SaveDirectory: *saveDirectory,
	}
}

func (i IncrementalBackup) Backup(manifest sstable.Manifest) error {
	//TODO implement me
	panic("implement me")
}

func (i IncrementalBackup) Restore() error {
	//TODO implement me
	panic("implement me")
}

func (i IncrementalBackup) GetInfo() BackupInfo {
	return i.info
}
