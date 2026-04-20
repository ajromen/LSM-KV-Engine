package backup

import (
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
)

type BackupManager struct {
	Backups []IBackup
}

func NewBackupManager() *BackupManager {
	return &BackupManager{}
}

func (bm *BackupManager) CreateBackup() {
	var backup IBackup
	switch config.GetSettings().Backup.Type {
	case enums.IncrementalBackup:
		backup = NewIncrementalBackup(nil)
	case enums.FullBackup:
		backup = NewFullBackup(nil)
	}

}

func (bm *BackupManager) addBackup(backup IBackup) {
	bm.Backups = append(bm.Backups, backup)
}
