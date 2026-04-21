package backup

import "github.com/ajromen/LSM-KV-Engine/internal/sstable"

const BackupFileExtension = ".backup"

type IBackup interface {
	Backup(manifest sstable.Manifest) error
	GetInfo() *BackupInfo
	GetId() string
	Restore(directory string) error
	ContainsFile(filename string) bool
}
