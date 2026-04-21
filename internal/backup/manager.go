package backup

import (
	"fmt"
	"path"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
	"github.com/ajromen/LSM-KV-Engine/internal/sstable"
)

type BackupManager struct {
	Backups    map[string]IBackup
	Manifest   *BackupManifest
	LastBackup *IBackup
}

func NewBackupManager() (*BackupManager, error) {
	settings := config.GetSettings()
	mainDir := path.Join(settings.SavePath, settings.Backup.SaveDirectory)
	manifest, err := NewBackupManifest(mainDir)
	if err != nil {
		return nil, err
	}
	backupManager := &BackupManager{
		Manifest: manifest,
		Backups:  make(map[string]IBackup),
	}

	err = backupManager.restoreManagerFromManifest()
	if err != nil {
		return nil, err
	}

	return backupManager, nil
}

func (bm *BackupManager) restoreManagerFromManifest() error {
	for _, entry := range bm.Manifest.Backups {
		var backup IBackup
		switch entry.Type {
		case enums.FullBackup:
			backup = NewFullBackup(&entry.BackupDirectory)
		case enums.IncrementalBackup:
			backup = NewIncrementalBackup(&entry.BackupDirectory, nil)
		}

		bm.Backups[backup.GetId()] = backup
		bm.LastBackup = &backup
	}

	if config.GetSettings().Debug {
		fmt.Printf("Loaded %d backups\n", len(bm.Backups))
	}

	return nil
}

func (bm *BackupManager) CreateBackup(manifest sstable.Manifest, backupType enums.BackupType) (string, error) {
	var backup IBackup
	switch backupType {
	case enums.IncrementalBackup:
		if bm.LastBackup == nil {
			backup = NewFullBackup(nil)
			break
		}
		backup = NewIncrementalBackup(nil, bm.LastBackup)
		info := backup.GetInfo()
		info.Base = bm.GetById(info.Id)
	case enums.FullBackup:
		backup = NewFullBackup(nil)
	}

	err := backup.Backup(manifest)
	if err != nil {
		return "", err
	}
	err = bm.addBackup(backup)
	if err != nil {
		return "", err
	}

	return backup.GetId(), nil
}

func (bm *BackupManager) addBackup(backup IBackup) error {
	bm.Backups[backup.GetId()] = backup
	bm.LastBackup = &backup
	return bm.Manifest.AddBackup(backup.GetInfo())
}

func (bm *BackupManager) GetById(id string) *IBackup {
	backup, ok := bm.Backups[id]
	if !ok {
		return nil
	}
	return &backup
}

func (bm *BackupManager) GetAll() []BackupInfo {
	var backups []BackupInfo
	for _, backup := range bm.Backups {
		backups = append(backups, *backup.GetInfo())
	}
	return backups
}

func (bm *BackupManager) DeleteBackup(id string) error {
	backup, ok := bm.Backups[id]
	if !ok {
		return fmt.Errorf("cannot delete: backup %s doesnt exist", id)
	}
	for _, b := range bm.Backups {
		if b.GetInfo().BaseId == id {
			return fmt.Errorf("cannot delete: backup %s depends on it", b.GetInfo().Id)
		}
	}
	err := block.DeleteDirectory(backup.GetInfo().SaveDirectory)
	if err != nil {
		return err
	}
	delete(bm.Backups, id)
	return bm.Manifest.RemoveBackup(backup.GetInfo().SaveDirectory)
}

func (bm *BackupManager) RestoreFrom(backup *IBackup, toDirectory string) error {
	err := (*backup).Restore(toDirectory)
	if err != nil {
		return err
	}
	return nil
}

func (bm *BackupManager) CascadeDelete(id string) error {
	backup, ok := bm.Backups[id]
	if !ok {
		return fmt.Errorf("cannot delete: backup %s doesnt exist", id)
	}
	for _, b := range bm.Backups {
		if b.GetInfo().BaseId == id {
			err := bm.CascadeDelete(b.GetInfo().Id)
			if err != nil {
				return err
			}
		}
	}

	err := block.DeleteDirectory(backup.GetInfo().SaveDirectory)
	if err != nil {
		return err
	}
	delete(bm.Backups, id)
	return bm.Manifest.RemoveBackup(backup.GetInfo().SaveDirectory)
}
