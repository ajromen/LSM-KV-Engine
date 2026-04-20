package backup

import "github.com/ajromen/LSM-KV-Engine/internal/sstable"

type IBackup interface {
	Backup(manifest sstable.Manifest) error
	Restore() error
	GetInfo() error
}
