package backup

import (
	"github.com/ajromen/LSM-KV-Engine/internal/sstable"
	"github.com/ajromen/LSM-KV-Engine/internal/wal"
)

const BackupFileExtension = ".backup"

type IBackup interface {
	Backup(manifest sstable.Manifest, walManifest wal.WALManifest) error
	GetInfo() *BackupInfo
	GetId() string
	Restore(directory string) error
	ContainsFile(filename string) bool
}
