package cli

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"

	"github.com/ajromen/LSM-KV-Engine/internal/core"
)

func handleRangeScan(engine *core.Engine, parts []string, reader *bufio.Reader) {
	if len(parts) != 4 {
		PrintError("Usage: range-scan <lower> <upper> <pageSize>")
		return
	}
	lower := parts[1]
	upper := parts[2]
	pageSize, err := strconv.Atoi(parts[3])
	if err != nil || pageSize <= 0 {
		PrintError("Invalid pageSize")
		return
	}

	scan, err := engine.NewPagedRangeScan(lower, upper, pageSize)
	if err != nil {
		PrintError(fmt.Sprintf("RangeScan error: %v", err))
		return
	}
	defer scan.Close()

	PrintSuccess("Range scan started. Commands: next/n  prev/p  stop/s")
	printScanResults(scan.CurrentPage())

	for {
		printReady(fmt.Sprintf("scan p%d", scan.PageNumber()))
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)

		switch line {
		case "next", "n":
			results := scan.NextPage()
			if scan.PageNumber() > 0 && len(results) == 0 {
				PrintError("(no more pages)")
			} else {
				printScanResults(results, false)
			}
		case "prev", "p":
			printScanResults(scan.PrevPage(), false)
		case "stop", "s":
			PrintSuccess("Scan stopped.")
			return
		default:
			PrintError("Unknown command. Use: next/n  prev/p  stop/s")
		}

		if scan.IsDirty() {
			PrintError("[!] Data on this page has changed.")
			results, _ := scan.CurrentPage()
			printScanResults(results, true)
		}
	}
}

func handlePrefixScan(engine *core.Engine, parts []string, reader *bufio.Reader) {
	if len(parts) != 3 {
		PrintError("Usage: prefix-scan <prefix> <pageSize>")
		return
	}
	prefix := parts[1]
	pageSize, err := strconv.Atoi(parts[2])
	if err != nil || pageSize <= 0 {
		PrintError("Invalid pageSize")
		return
	}

	scan, err := engine.NewPagedPrefixScan(prefix, pageSize)
	if err != nil {
		PrintError(fmt.Sprintf("PrefixScan error: %v", err))
		return
	}
	defer scan.Close()

	PrintSuccess("Prefix scan started. Commands: next/n  prev/p  stop/s")
	printScanResults(scan.CurrentPage())

	for {
		printReady(fmt.Sprintf("scan p%d", scan.PageNumber()))
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)

		switch line {
		case "next", "n":
			results := scan.NextPage()
			if scan.PageNumber() > 0 && len(results) == 0 {
				PrintError("(no more pages)")
			} else {
				printScanResults(results, false)
			}
		case "prev", "p":
			printScanResults(scan.PrevPage(), false)
		case "stop", "s":
			PrintSuccess("Scan stopped.")
			return
		default:
			PrintError("Unknown command. Use: next/n  prev/p  stop/s")
		}

		if scan.IsDirty() {
			PrintError("[!] Data on this page has changed.")
			results, _ := scan.CurrentPage()
			printScanResults(results, true)
		}
	}
}

func printScanResults(results []core.ScanResult, refreshed bool) {
	if refreshed {
		PrintSuccess("--- refreshed ---")
	}
	if len(results) == 0 {
		PrintError("(no results)")
		return
	}
	for _, r := range results {
		PrintSuccess(fmt.Sprintf("%s -> %s", r.Key, r.Value))
	}
}
