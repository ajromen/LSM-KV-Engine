package backup

import (
	"path"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
)

type BackupManager struct {
	Backups  []IBackup
	Manifest *BackupManifest
}

func NewBackupManager() (*BackupManager, error) {
	settings := config.GetSettings()
	mainDir := path.Join(settings.SavePath, settings.Backup.SaveDirectory)
	manifest, err := NewBackupManifest(mainDir)
	if err != nil {
		return nil, err
	}
	return &BackupManager{
		Manifest: manifest,
		Backups:  make([]IBackup, 0),
	}, nil
}

func (bm *BackupManager) CreateBackup() {
	var backup IBackup
	switch config.GetSettings().Backup.Type {
	case enums.IncrementalBackup:
		backup = NewIncrementalBackup(nil)
	case enums.FullBackup:
		backup = NewFullBackup(nil)
	}
	bm.addBackup(backup)
}

func (bm *BackupManager) addBackup(backup IBackup) {
	bm.Backups = append(bm.Backups, backup)
	bm.Manifest.AddBackup(backup.GetInfo())
}
