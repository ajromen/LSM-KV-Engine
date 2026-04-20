package backup

import "github.com/ajromen/LSM-KV-Engine/internal/sstable"

type IncrementalBackup struct {
	info          BackupInfo
	SaveDirectory string
}

func NewIncrementalBackup(saveDirectory *string) *IncrementalBackup {
	return &IncrementalBackup{}
}

func (i IncrementalBackup) Backup(manifest sstable.Manifest) error {
	//TODO implement me
	panic("implement me")
}

func (i IncrementalBackup) Restore() error {
	//TODO implement me
	panic("implement me")
}

func (i IncrementalBackup) GetInfo() error {
	//TODO implement me
	panic("implement me")
}
