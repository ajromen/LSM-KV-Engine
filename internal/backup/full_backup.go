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

type FullBackup struct {
	info BackupInfo
}

// leave saveDirectory as nil if directory doesnt exist yet
func NewFullBackup(saveDirectory *string) *FullBackup {
	settings := config.GetSettings()
	var info BackupInfo

	if saveDirectory == nil {
		timestamp := time.Now().Unix()
		id := strconv.FormatInt(timestamp, 10)
		savePath := path.Join(settings.SavePath, settings.Backup.SaveDirectory, id)
		saveDirectory = &savePath
		info = BackupInfo{
			Id:            id,
			BaseId:        "",
			Type:          enums.FullBackup,
			Timestamp:     timestamp,
			Files:         nil,
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

	return &FullBackup{
		info: info,
	}
}

func (f *FullBackup) Backup(manifest sstable.Manifest, walManifest wal.WALManifest) error {
	err := block.EnsureDir(f.info.SaveDirectory)
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
		src := filepath.Join(dir, name)
		dst := filepath.Join(f.info.SaveDirectory, name)
		if err := block.CopyFile(src, dst); err != nil {
			return fmt.Errorf("failed to copy %s: %w", name, err)
		}
		fileNames = append(fileNames, name)
	}

	// wal fajlovi
	walDstDir := filepath.Join(f.info.SaveDirectory, "wal")
	if err := block.EnsureDir(walDstDir); err != nil {
		return fmt.Errorf("failed to create wal backup dir: %w", err)
	}

	for _, seg := range walManifest.SortedSegments() {
		//skip fully flushed
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

	// wal manifest
	walManifestSrc := filepath.Join(walManifest.FileDir, wal.ManifestFileName)
	walManifestDst := filepath.Join(walDstDir, wal.ManifestFileName)
	if err := block.CopyFile(walManifestSrc, walManifestDst); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to copy wal manifest: %w", err)
	}

	f.info.Files = fileNames
	if err := f.info.SaveToFile(filepath.Join(f.info.SaveDirectory, InfoFileName)); err != nil {
		return err
	}
	return manifest.SaveTo(f.info.SaveDirectory)
}

func (f *FullBackup) Restore(directory string) error {
	// sstable
	for _, file := range f.info.Files {
		if strings.HasPrefix(file, "wal"+string(os.PathSeparator)) {
			continue // wal se restore-uje posebno
		}
		filePath := path.Join(f.info.SaveDirectory, file)
		destPath := path.Join(directory, filepath.Base(file))
		if err := block.CopyFile(filePath, destPath); err != nil {
			return err
		}
	}

	// sstable manifest
	destManifest := path.Join(directory, sstable.ManifestFileName)
	if err := block.CopyFile(path.Join(f.info.SaveDirectory, sstable.ManifestFileName), destManifest); err != nil {
		return fmt.Errorf("failed to restore sstable manifest: %w", err)
	}

	// wal
	settings := config.GetSettings()
	walDstDir := filepath.Join(directory, settings.WAL.SaveDirectory)
	if err := block.EnsureDir(walDstDir); err != nil {
		return fmt.Errorf("failed to create wal restore dir: %w", err)
	}

	walSrcDir := filepath.Join(f.info.SaveDirectory, "wal")
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
func (f *FullBackup) GetInfo() *BackupInfo {
	return &f.info
}

func (f *FullBackup) GetId() string {
	return f.info.Id
}

func (f *FullBackup) ContainsFile(filename string) bool {
	for _, file := range f.info.Files {
		if file == filename {
			return true
		}
	}
	return false
}
