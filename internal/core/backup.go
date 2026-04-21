package core

import (
	"fmt"
	"time"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
)

func (engine *Engine) GetAllBackups() []string {
	var formated []string
	backups := engine.backupManager.GetAll()
	for _, backup := range backups {
		var backupType string
		switch backup.Type {
		case enums.FullBackup:
			backupType = "full"
		case enums.IncrementalBackup:
			backupType = "incremental"
		default:
			backupType = "unknown"
		}

		formated = append(formated, fmt.Sprintf("%d    %s    %s", backup.Id, backupType, time.Unix(backup.Timestamp, 0).String()))
	}
	return formated
}

// steps:
// 1. create full backup so you can restore if anything happens
// 2. delete everything
// 3. try restore - if restore fails restore from full backup
// 4. remove full backup
func (engine *Engine) RestoreFromBackup(backupId string) error {
	backup := engine.backupManager.GetById(backupId)
	if backup == nil {
		return fmt.Errorf("backup %s not found", backupId)
	}

	// step 1
	newBackupId, err := engine.CreateBackup(enums.FullBackup)
	if err != nil {
		return err
	}

	// step 2
	err = engine.lsm.Finish()
	if err != nil {
		return err
	}

	err = engine.ClearAll()
	if err != nil {
		_ = engine.backupManager.DeleteBackup(newBackupId)
		return err
	}

	// step 3
	err = engine.backupManager.RestoreFrom(backup, config.GetSettings().SavePath)
	if err != nil {
		newBackup := engine.backupManager.GetById(newBackupId)
		_ = engine.backupManager.RestoreFrom(newBackup, config.GetSettings().SavePath)
		_ = engine.backupManager.DeleteBackup(newBackupId)
		return fmt.Errorf("unable to restore backup %s: %w \n restored to previous state ", backupId, err)
	}

	//step 4
	_ = engine.backupManager.DeleteBackup(newBackupId)

	return nil
}

func (engine *Engine) CreateBackup(backupType enums.BackupType) (string, error) {
	id, err := engine.backupManager.CreateBackup(engine.lsm.GetManifest(), backupType)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (engine *Engine) DeleteBackup(backupId string) error {
	return engine.backupManager.DeleteBackup(backupId)
}

func (engine *Engine) CreateSnapshot() error {
	return nil
}

func (engine *Engine) CheckoutSnapshot() error {
	return nil
}
