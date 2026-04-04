package cli

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/ajromen/LSM-KV-Engine/internal/core"
)

func handleRangeIterate(engine *core.Engine, parts []string, reader *bufio.Reader) {
	if len(parts) != 3 {
		PrintError("Usage: range-iterate <lower> <upper>\n")
		return
	}
	lower := parts[1]
	upper := parts[2]

	it, err := engine.RangeIterate(lower, upper)
	if err != nil {
		PrintSuccess(fmt.Sprintln("RangeIterate error:", err))
		return
	}

	if !it.Valid() {
		PrintError("No results in range\n")
		return
	}

	PrintSuccess("Iterator started. Type 'next' for next entry, 'stop' to exit.")
	runIteratorLoop(it, reader)
}

func handlePrefixIterate(engine *core.Engine, parts []string, reader *bufio.Reader) {
	if len(parts) != 2 {
		PrintError("Usage: prefix-iterate <prefix>")
		return
	}
	prefix := parts[1]

	it, err := engine.PrefixIterate(prefix)
	if err != nil {
		PrintError(fmt.Sprintln("PrefixIterate error:", err))
		return
	}

	if !it.Valid() {
		PrintError("No results with prefix\n")
		return
	}

	PrintSuccess("Iterator started. Type 'next/n' for next entry, 'stop/s' to exit.\n")
	runIteratorLoop(it, reader)
}

func runIteratorLoop(it *core.ActiveIterator, reader *bufio.Reader) {
	for {
		printReady("iter")
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)

		switch line {
		case "next", "n":
			result, ok := it.Next()
			if !ok {
				PrintSuccess("End of iterator")
				return
			}
			PrintSuccess(fmt.Sprintf("%s -> %s", result.Key, result.Value))
		case "stop", "s":
			PrintSuccess("Iterator stopped.")
			return
		default:
			PrintError("Unknown iterator command. Use 'next/n' or 'stop/s'.\n")
		}
	}
}
