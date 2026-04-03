package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

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
			handleRangeScan(engine, parts)
		case "prefix-scan":
			handlePrefixScan(engine, parts)
		case "range-iterate":
			handleRangeIterate(engine, parts, reader)
		case "prefix-iterate":
			handlePrefixIterate(engine, parts, reader)
		case "exit", "quit", "q":
			print("Exiting...")
			err := engine.Close()
			if err != nil {
				PrintError(fmt.Sprint("Closing error: ", err))
				os.Exit(1)
			}
			os.Exit(0)
		case "help":
			PrintSuccess(fmt.Sprint(helpText))
		case "dataraw":
			handleDataRaw(engine, parts)
		default:
			PrintError(fmt.Sprint("Unknown command: ", parts[0]))
		}
	}
}
