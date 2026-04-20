package backup

import "github.com/ajromen/LSM-KV-Engine/internal/sstable"

const BackupFileExtension = ".backup"

type IBackup interface {
	Backup(manifest sstable.Manifest) error
	Restore() error
	GetInfo() BackupInfo
}
