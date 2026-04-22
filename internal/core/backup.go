package core

import (
	"encoding/base64"
	"fmt"
	"time"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
)

func (engine *Engine) GetAllBackups() []string {
	var formated []string
	backups := engine.backupManager.GetAll()
	formated = append(formated, "    Id      |  Type  |      Created At")
	for _, backup := range backups {
		var backupType string
		switch backup.Type {
		case enums.FullBackup:
			backupType = "full"
		case enums.IncrementalBackup:
			backupType = "incr"
		case enums.Checkpoint:
			backupType = "chck"
		default:
			backupType = "unknown"
		}
		encoded := base64.StdEncoding.EncodeToString([]byte(backup.Id))
		copySeq := fmt.Sprintf("\033]52;c;%s\a", encoded)
		formated = append(formated, fmt.Sprintf(" \033[4m%s\033[24m%s    %s    %s",
			backup.Id,
			copySeq,
			backupType,
			time.Unix(backup.Timestamp, 0).Format("15:04:05 02 Jan 2006")))
	}
	return formated
}

// steps:
// 1. create full backup so you can restore if anything happens
// 2. delete everything
// 3. try restore - if restore fails restore from full backup
// 4. remove full backup
// 5. reload lsm
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
	// wal
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

	// step 4
	if err := engine.reinitialize(); err != nil {
		return fmt.Errorf("restore ok but failed to reinitialize: %w", err)
	}

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

func (engine *Engine) CascadeDeleteBackup(backupId string) error {
	return engine.backupManager.CascadeDelete(backupId)
}

func (engine *Engine) DeleteAllBackups() error {
	return engine.backupManager.DeleteAllBackups()
}
