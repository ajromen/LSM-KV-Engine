package cli

import (
	"fmt"
	"strconv"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/core"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
)

func handleListBackups(engine *core.Engine, parts []string) {
	if len(parts) != 1 {
		PrintError(fmt.Sprint("Usage: list-backups"))
		return
	}

	backups := engine.GetAllBackups()

	if len(backups) == 1 {
		PrintError("No backups found")
		return
	}

	PrintSpecial(backups[0])
	for _, backup := range backups[1:] {
		PrintSuccess(backup)
	}
}

func handleCreateBackup(engine *core.Engine, parts []string) {
	if len(parts) > 2 {
		PrintError(fmt.Sprint("Usage: create-backup [type]"))
		return
	}

	if len(parts) == 1 {
		id, err := engine.CreateBackup(config.GetSettings().Backup.Type)
		if err != nil {
			PrintError(fmt.Sprint("Create backup failed: ", err.Error()))
		}
		PrintSuccess(fmt.Sprintf("Created backup %s", id))
		return
	}

	var backupType enums.BackupType
	switch parts[1] {
	case "full":
		backupType = enums.FullBackup
	case "incremental":
		backupType = enums.IncrementalBackup
	case "checkpoint":
		backupType = enums.Checkpoint
	}

	id, err := engine.CreateBackup(backupType)
	if err != nil {
		PrintError(fmt.Sprint("Create backup failed: ", err.Error()))
	}
	PrintSuccess(fmt.Sprintf("Created backup %s", id))
}

func handleDeleteBackup(engine *core.Engine, parts []string) {
	if len(parts) != 2 {
		PrintError(fmt.Sprint("Usage: delete-backup <id>"))
		return
	}

	err := engine.DeleteBackup(parts[1])
	if err != nil {
		PrintError(fmt.Sprint("Delete backup failed: ", err.Error()))
		PrintError("Try using cascade-delete-backup")
		return
	}

	PrintSuccess(fmt.Sprintf("Deleted backup %s", parts[1]))
}

func handleCascadeDeleteBackup(engine *core.Engine, parts []string) {
	if len(parts) != 2 {
		PrintError(fmt.Sprint("Usage: cascade-delete-backup <id>"))
		return
	}

	err := engine.CascadeDeleteBackup(parts[1])
	if err != nil {
		PrintError(fmt.Sprint("Cascade delete backup failed: ", err.Error()))
		return
	}
	PrintSuccess(fmt.Sprintf("Cascade deleted backup successful %s", parts[1]))
}

func handleRestoreBackup(engine *core.Engine, parts []string) {
	if len(parts) != 2 {
		PrintError(fmt.Sprint("Usage: restore-backup <id>"))
		return
	}

	err := engine.RestoreFromBackup(parts[1])
	if err != nil {
		PrintError(fmt.Sprintf("Restore unsuccessful: %s", err))
		return
	}
	PrintSuccess("Restore successful")
}

func handleDeleteAllBackups(engine *core.Engine, parts []string) {
	if len(parts) != 1 {
		PrintError(fmt.Sprint("Usage: delete-all-backups"))
		return
	}

	err := engine.DeleteAllBackups()
	if err != nil {
		PrintError(fmt.Sprintf("Delete all backups failed: %s", err))
		return
	}
	PrintSuccess("Delete all backups successful")
}

func handleSnapshot(engine *core.Engine, parts []string) {
	if len(parts) != 2 {
		PrintError("Usage: snapshot <key>")
		return
	}
	engine.Snapshot([]byte(parts[1]))
	PrintSuccess(fmt.Sprintf("Snapshot: key '%s' will now retain all versions", parts[1]))
}

func handleGetVersions(engine *core.Engine, parts []string) {
	if len(parts) != 2 {
		PrintError("Usage: get-versions <key>")
		return
	}
	key := parts[1]
	versions, err := engine.GetVersions([]byte(key))
	if err != nil {
		PrintError(fmt.Sprint("GetVersions:", err))
		return
	}
	if len(versions) == 0 {
		PrintError(fmt.Sprintf("No versions found for key '%s'", key))
		return
	}
	for i, v := range versions {
		label := "current"
		if i > 0 {
			label = fmt.Sprintf("v-%d", i)
		}
		PrintSuccess(fmt.Sprintf("[%s] %s", label, v))
	}
}

func handleGetVersion(engine *core.Engine, parts []string) {
	if len(parts) != 3 {
		PrintError("Usage: get-version <key> <version>")
		return
	}
	key := parts[1]
	n, err := strconv.Atoi(parts[2])
	if err != nil || n < 0 {
		PrintError("Version must be a non-negative integer (0 = current)")
		return
	}
	value, found, err := engine.GetVersion([]byte(key), n)
	if err != nil {
		PrintError(fmt.Sprint("GetVersion:", err))
		return
	}
	if !found {
		PrintError(fmt.Sprintf("Version %d not found for key '%s'", n, key))
		return
	}
	PrintSuccess(value)
}
