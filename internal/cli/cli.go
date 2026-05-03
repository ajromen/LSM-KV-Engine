package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/core"
)

func RunCli(engine *core.Engine) {
	reader := bufio.NewReader(os.Stdin)
	printBanner()
	for {
		printReady("")

		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		cmd := parts[0]

		switch cmd {
		case "put":
			handlePut(engine, parts)
		case "del":
			handleDelete(engine, parts)
		case "get":
			handleGet(engine, parts)
		case "expire":
			handleExpire(engine, parts)
		case "ttl":
			handleTTL(engine, parts)
		case "del-range":
			handleRangeDel(engine, parts)
		case "clear-all":
			handleClear(engine, parts)
		case "range-scan":
			handleRangeScan(engine, parts, reader)
		case "prefix-scan":
			handlePrefixScan(engine, parts, reader)
		case "range-iterate":
			handleRangeIterate(engine, parts, reader)
		case "prefix-iterate":
			handlePrefixIterate(engine, parts, reader)
		case "snapshot":
			handleSnapshot(engine, parts)
		case "get-versions":
			handleGetVersions(engine, parts)
		case "get-version":
			handleGetVersion(engine, parts)
		case "delete-backup":
			handleDeleteBackup(engine, parts)
		case "create-backup":
			handleCreateBackup(engine, parts)
		case "list-backups":
			handleListBackups(engine, parts)
		case "cascade-delete-backup":
			handleCascadeDeleteBackup(engine, parts)
		case "delete-all-backups":
			handleDeleteAllBackups(engine, parts)
		case "restore-backup":
			handleRestoreBackup(engine, parts)
		case "batch-write":
			handleBatchWrite(engine, parts)
		case "batch-delete":
			handleBatchDelete(engine, parts)
		case "bf-create":
			handleBFCreate(engine, parts)
		case "bf-add":
			handleBFAdd(engine, parts)
		case "bf-contains":
			handleBFContains(engine, parts)
		case "bf-delete":
			handleBFDelete(engine, parts)
		case "cms-create":
			handleCMSCreate(engine, parts)
		case "cms-add":
			handleCMSAdd(engine, parts)
		case "cms-freq":
			handleCMSFreq(engine, parts)
		case "cms-delete":
			handleCMSDelete(engine, parts)
		case "hll-create":
			handleHLLCreate(engine, parts)
		case "hll-add":
			handleHLLAdd(engine, parts)
		case "hll-count":
			handleHLLCount(engine, parts)
		case "hll-delete":
			handleHLLDelete(engine, parts)
		case "sh-store":
			handleSHStore(engine, parts)
		case "sh-dist":
			handleSHDist(engine, parts)
		case "sh-delete":
			handleSHDelete(engine, parts)
		case "exit", "quit", "q":
			print(blue + "Exiting...")
			err := engine.Close()
			if err != nil {
				PrintError(fmt.Sprint("Closing error: ", err))
				os.Exit(1)
			}
			os.Exit(0)
		case "help":
			printHelpMessage()
		case "help-probabilistic":
			printHelpProbabilisticMessage()
		case "dataraw":
			if config.GetSettings().Debug {
				handleDataRaw(engine, parts)
				continue
			}
			PrintError(fmt.Sprint("Unknown command: ", parts[0]))
		case "print-wal":
			if config.GetSettings().Debug {
				handlePrintWal(engine)
				continue
			}
			PrintError(fmt.Sprint("Unknown command: ", parts[0]))
		default:
			PrintError(fmt.Sprint("Unknown command: ", parts[0]))
		}
	}
}
