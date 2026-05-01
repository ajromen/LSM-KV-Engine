package backup

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ajromen/LSM-KV-Engine/internal/block"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
	"github.com/ajromen/LSM-KV-Engine/internal/sstable"
	"github.com/ajromen/LSM-KV-Engine/internal/wal"
)

type IncrementalBackup struct {
	info BackupInfo
}

func NewIncrementalBackup(saveDirectory *string, baseBackup *IBackup) *IncrementalBackup {
	settings := config.GetSettings()
	var info BackupInfo

	if saveDirectory == nil {
		timestamp := time.Now().Unix()
		id := strconv.FormatInt(timestamp, 10)
		savePath := path.Join(settings.SavePath, settings.Backup.SaveDirectory, id)
		saveDirectory = &savePath

		baseId := ""
		if baseBackup != nil {
			baseId = (*baseBackup).GetId()
		}

		info = BackupInfo{
			Id:            id,
			BaseId:        baseId,
			Base:          baseBackup,
			Type:          enums.IncrementalBackup,
			Timestamp:     timestamp,
			Files:         nil,
			NewFiles:      nil,
			SaveDirectory: *saveDirectory,
		}
	} else {
		info = BackupInfo{}
		err := info.LoadFromFile(path.Join(*saveDirectory, InfoFileName))
		info.SaveDirectory = *saveDirectory
		if err != nil {
			panic(err)
		}
	}
	return &IncrementalBackup{
		info: info,
	}
}

func (i *IncrementalBackup) Backup(manifest sstable.Manifest, walManifest wal.WALManifest) error {
	err := block.EnsureDir(i.info.SaveDirectory)
	if err != nil {
		return err
	}

	// sstable
	baseNames := make(map[string]struct{})
	for _, layer := range manifest.Layers {
		for _, sst := range layer {
			baseNames[filepath.Base(sst.BaseFileName)] = struct{}{}
		}
	}
	dir := manifest.FileDir
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read data dir: %w", err)
	}
	var fileNames []string
	var unsavedFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == sstable.ManifestFileName {
			continue
		}
		matched := false
		for base := range baseNames {
			if name == base || strings.HasPrefix(name, base+".") {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		fileNames = append(fileNames, name)
		if i.ContainsFile(name) {
			continue
		}
		src := filepath.Join(dir, name)
		dst := filepath.Join(i.info.SaveDirectory, name)
		if err := block.CopyFile(src, dst); err != nil {
			return fmt.Errorf("failed to copy %s: %w", name, err)
		}
		unsavedFiles = append(unsavedFiles, name)
	}

	// wal
	walDstDir := filepath.Join(i.info.SaveDirectory, "wal")
	if err := block.EnsureDir(walDstDir); err != nil {
		return fmt.Errorf("failed to create wal backup dir: %w", err)
	}

	for _, seg := range walManifest.SortedSegments() {
		if seg.ValidFromBlock >= uint64(walManifest.MaxBlocks()) {
			continue
		}
		segName := fmt.Sprintf("wal_%06d.log", seg.SegmentID)
		src := filepath.Join(walManifest.FileDir, segName)
		dst := filepath.Join(walDstDir, segName)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}
		if err := block.CopyFile(src, dst); err != nil {
			return fmt.Errorf("failed to copy wal segment %s: %w", segName, err)
		}
		fileNames = append(fileNames, filepath.Join("wal", segName))
	}

	walManifestSrc := filepath.Join(walManifest.FileDir, wal.ManifestFileName)
	walManifestDst := filepath.Join(walDstDir, wal.ManifestFileName)
	if err := block.CopyFile(walManifestSrc, walManifestDst); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to copy wal manifest: %w", err)
	}

	i.info.Files = fileNames
	i.info.NewFiles = unsavedFiles
	if err := i.info.SaveToFile(filepath.Join(i.info.SaveDirectory, InfoFileName)); err != nil {
		return err
	}
	return manifest.SaveTo(i.info.SaveDirectory)
}

func (i *IncrementalBackup) Restore(directory string) error {
	if i.info.Base == nil {
		return fmt.Errorf("incremental backup %s has no base", i.info.Id)
	}
	if err := (*i.info.Base).Restore(directory); err != nil {
		return fmt.Errorf("failed to restore base backup: %w", err)
	}

	// sstable
	for _, file := range i.info.NewFiles {
		if strings.HasPrefix(file, "wal"+string(os.PathSeparator)) {
			continue
		}
		filePath := path.Join(i.info.SaveDirectory, file)
		destPath := path.Join(directory, filepath.Base(file))
		if err := block.CopyFile(filePath, destPath); err != nil {
			return err
		}
	}

	// sstable manifest
	destManifest := path.Join(directory, sstable.ManifestFileName)
	if err := block.CopyFile(path.Join(i.info.SaveDirectory, sstable.ManifestFileName), destManifest); err != nil {
		return fmt.Errorf("failed to restore manifest: %w", err)
	}

	//wal
	settings := config.GetSettings()
	walDstDir := filepath.Join(directory, settings.WAL.SaveDirectory)
	if err := block.EnsureDir(walDstDir); err != nil {
		return fmt.Errorf("failed to create wal restore dir: %w", err)
	}

	walSrcDir := filepath.Join(i.info.SaveDirectory, "wal")
	walEntries, err := os.ReadDir(walSrcDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read wal backup dir: %w", err)
	}
	for _, entry := range walEntries {
		if entry.IsDir() {
			continue
		}
		src := filepath.Join(walSrcDir, entry.Name())
		dst := filepath.Join(walDstDir, entry.Name())
		if err := block.CopyFile(src, dst); err != nil {
			return fmt.Errorf("failed to restore wal file %s: %w", entry.Name(), err)
		}
	}

	return nil
}

func (i *IncrementalBackup) GetInfo() *BackupInfo {
	return &i.info
}

func (i *IncrementalBackup) GetId() string {
	return i.info.Id
}

func (i *IncrementalBackup) ContainsFile(fileName string) bool {
	for _, file := range i.info.NewFiles {
		if file == fileName {
			return true
		}
	}

	if i.info.Base == nil {
		return false
	}

	// check if some lover level contains file
	return (*i.info.Base).ContainsFile(fileName)
}
