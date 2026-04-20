package backup

import "github.com/ajromen/LSM-KV-Engine/internal/enums"

const ManifestFileName = "MANIFEST.json"

type BackupManifestEntry struct {
	FilePath  string           `json:"file_path"`
	Timestamp string           `json:"timestamp"`
	Type      enums.BackupType `json:"type"`
}

type BackupManifest struct {
	Backups []BackupManifestEntry `json:"backups"`
}

func NewBackupManifest() *BackupManifest {
	return &BackupManifest{}
}

func (bm *BackupManifest) reconstruct(fileDir string) {

}
