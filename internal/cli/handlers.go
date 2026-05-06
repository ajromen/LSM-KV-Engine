package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/ajromen/LSM-KV-Engine/internal/core"
)

func handlePut(engine *core.Engine, parts []string) {
	if len(parts) < 3 || len(parts) > 4 {
		PrintError(fmt.Sprint("Usage: put <key> <value> [ttl]"))
		return
	}

	key := parts[1]
	value := parts[2]

	if len(parts) == 4 {
		ttl, err := parseTTL(parts[3])
		if err != nil {
			PrintError(fmt.Sprint("TTL must be a number"))
			return
		}
		engine.PutWithTTL([]byte(key), []byte(value), ttl)
	} else {
		engine.Put([]byte(key), []byte(value))
	}

	PrintSuccess(fmt.Sprint("Put: ", key, " OK"))
}

func handleGet(engine *core.Engine, parts []string) {
	if len(parts) != 2 {
		PrintError(fmt.Sprint("Usage: get <key>"))
		return
	}
	key := parts[1]
	value, found, err := engine.Get([]byte(key))
	if err != nil {
		PrintError(fmt.Sprint("Get:", err))
		return
	}
	if !found {
		PrintError(fmt.Sprint("Get: key '" + key + "' not found"))
		return
	}
	PrintSuccess(fmt.Sprint(string(value)))
}

func handleDelete(engine *core.Engine, parts []string) {
	if len(parts) != 2 {
		PrintError(fmt.Sprint("Usage: delete <key>"))
		return
	}
	key := parts[1]
	engine.Delete([]byte(key))

	PrintSuccess(fmt.Sprint("Delete: ", key, " OK"))
}

func handleRangeDel(engine *core.Engine, parts []string) {
	if len(parts) != 3 {
		PrintError(fmt.Sprint("Usage: del-range <key1> <key2>"))
	}
	startKey := parts[1]
	endKey := parts[2]
	engine.RangeDelete([]byte(startKey), []byte(endKey))
	PrintSuccess(fmt.Sprint("RangeDel: ", startKey, endKey, " OK"))
}

func handleClear(engine *core.Engine, parts []string) {
	if len(parts) != 1 {
		PrintError(fmt.Sprint("Usage: clear-all"))
		return
	}
	fmt.Print(red + "DELETE ALL DATA? (yes/N): " + reset)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(answer)
	if answer != "yes" {
		PrintError(fmt.Sprint("Aborted."))
		return
	}
	if err := engine.ClearAll(false); err != nil {
		PrintError(fmt.Sprint("ClearAll:", err))
		return
	}
	PrintSuccess(fmt.Sprint("ClearAll: OK"))
}
