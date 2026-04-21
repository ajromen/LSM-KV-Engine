package backup

import (
	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
)

const InfoFileName = "backup_metadata.json"

type BackupInfo struct {
	Id        string           `json:"id"`
	BaseId    string           `json:"base_id"`
	Type      enums.BackupType `json:"type"`
	Timestamp int64            `json:"timestamp"`
	Files     []string         `json:"files"`
	NewFiles  []string         `json:"new_files"` // za incremental

	Base          *IBackup `json:"-"` // runtime
	SaveDirectory string   `json:"-"` // runtime
}

func (bi *BackupInfo) SaveToFile(filename string) error {
	err := block.WriteJSON(filename, bi)
	return err
}

func (bi *BackupInfo) LoadFromFile(filename string) error {
	err := block.ReadJSON(filename, bi)
	return err
}
