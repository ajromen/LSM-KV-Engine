package backup

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
)

const ManifestFileName = "MANIFEST.json"

type BackupManifestEntry struct {
	FileName string `json:"filename"`
}

type BackupManifest struct {
	Backups []BackupManifestEntry `json:"backups"`
	FileDir string                `json:"file_path"`
}

func NewBackupManifest(fileDir string) (*BackupManifest, error) {
	manifest := &BackupManifest{
		FileDir: fileDir,
		Backups: make([]BackupManifestEntry, 0),
	}
	_, err := os.Stat(manifest.filePath())
	if err != nil {
		err = manifest.reconstruct(fileDir)
		if err != nil {
			return nil, err
		}
		if config.GetSettings().Debug {
			fmt.Printf("Reconstructed backup manifest from folder list\n")
		}
	} else {
		err := manifest.load()
		if err != nil {
			return nil, err
		}
		if config.GetSettings().Debug {
			fmt.Printf("Backup Manifest loaded from file \n")
		}
	}
	return manifest, nil
}

// read all directories in base dir and check if they contain info file
// if no skip else add to manifest
func (bm *BackupManifest) reconstruct(fileDir string) error {
	entries, err := os.ReadDir(fileDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			dirName := entry.Name()
			_, err := os.Stat(path.Join(dirName, InfoFileName))
			if err != nil {
				if config.GetSettings().Debug {
					fmt.Printf("Skipping backup invalid info file")
				}
				continue
			}
			bm.Backups = append(bm.Backups, BackupManifestEntry{FileName: dirName})
		}
	}

	err = bm.Save()
	return err
}

func (bm *BackupManifest) filePath() string {
	return filepath.Join(bm.FileDir, ManifestFileName)
}

func (bm *BackupManifest) Save() error {
	err := block.WriteJSON(bm.filePath(), bm)
	return err
}

func (bm *BackupManifest) load() error {
	err := block.ReadJSON(bm.filePath(), bm)
	if err != nil {
		return err
	}
	return nil
}

func (bm *BackupManifest) AddBackup(info BackupInfo) {
	bm.Backups = append(bm.Backups, BackupManifestEntry{FileName: info.})
}
