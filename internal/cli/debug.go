package cli

import (
	"fmt"
	"strconv"

	"github.com/ajromen/LSM-KV-Engine/internal/core"
)

func handleDataRaw(engine *core.Engine, parts []string) {
	if len(parts) != 2 {
		PrintError(fmt.Sprint("Usage: dataraw <index>"))
		return
	}

	indxStr := parts[1]

	indx, err := strconv.Atoi(indxStr)
	if err != nil {
		PrintError(fmt.Sprint("invalid index:", indxStr))
		return
	}

	engine.DataRaw(indx)
}

func handlePrintWal(engine *core.Engine) {
	err := engine.WalPrintAll()
	if err != nil {
		PrintError(fmt.Sprint("error printing wal:", err))
		return
	}
}
