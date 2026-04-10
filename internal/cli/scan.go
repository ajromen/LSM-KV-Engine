package cli

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/ajromen/LSM-KV-Engine/internal/core"
)

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

func handleRangeScan(engine *core.Engine, parts []string, reader *bufio.Reader) {
	if len(parts) != 4 {
		PrintError("Usage: range-scan <lower> <upper> <pageSize>")
		return
	}
	lower := parts[1]
	upper := parts[2]
	var pageSize int
	fmt.Sscan(parts[3], &pageSize)
	if pageSize <= 0 {
		PrintError("Invalid pageSize")
		return
	}

	scan, err := engine.NewPagedRangeScan(lower, upper, pageSize)
	if err != nil {
		PrintError(fmt.Sprintf("RangeScan error: %v", err))
		return
	}
	defer scan.Close()

	results, _ := scan.CurrentPage()
	renderScanPage(scan.PageNumber(), results, "")

	for {
		// provjeri dirty prije cekanja na input
		if scan.IsDirty() {
			results, _ = scan.CurrentPage()
			renderScanPage(scan.PageNumber(), results, "[!] Data on this page changed — auto refreshed")
		}

		printReady(fmt.Sprintf("scan p%d", scan.PageNumber()))
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)

		switch line {
		case "n":
			r, ok := scan.NextPage()
			if !ok {
				renderScanPage(scan.PageNumber(), results, "(no more pages)")
			} else {
				results = r
				renderScanPage(scan.PageNumber(), results, "")
			}
		case "p":
			results = scan.PrevPage()
			renderScanPage(scan.PageNumber(), results, "")
		case "s":
			clearScreen()
			PrintSuccess("Scan stopped.")
			return
		default:
			renderScanPage(scan.PageNumber(), results, "Unknown command. Use: n  p  s")
		}
	}
}

func handlePrefixScan(engine *core.Engine, parts []string, reader *bufio.Reader) {
	if len(parts) != 3 {
		PrintError("Usage: prefix-scan <prefix> <pageSize>")
		return
	}
	prefix := parts[1]
	var pageSize int
	fmt.Sscan(parts[2], &pageSize)
	if pageSize <= 0 {
		PrintError("Invalid pageSize")
		return
	}

	scan, err := engine.NewPagedPrefixScan(prefix, pageSize)
	if err != nil {
		PrintError(fmt.Sprintf("PrefixScan error: %v", err))
		return
	}
	defer scan.Close()

	results, _ := scan.CurrentPage()
	renderScanPage(scan.PageNumber(), results, "")

	for {
		if scan.IsDirty() {
			results, _ = scan.CurrentPage()
			renderScanPage(scan.PageNumber(), results, "[!] Data on this page changed — auto refreshed")
		}

		printReady(fmt.Sprintf("scan p%d", scan.PageNumber()))
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)

		switch line {
		case "n":
			r, ok := scan.NextPage()
			if !ok {
				renderScanPage(scan.PageNumber(), results, "(no more pages)")
			} else {
				results = r
				renderScanPage(scan.PageNumber(), results, "")
			}
		case "p":
			results = scan.PrevPage()
			renderScanPage(scan.PageNumber(), results, "")
		case "s":
			clearScreen()
			PrintSuccess("Scan stopped.")
			return
		default:
			renderScanPage(scan.PageNumber(), results, "Unknown command. Use: n  p  s")
		}
	}
}

// renderScanPage clears the screen and renders the current page
func renderScanPage(pageNum int, results []core.ScanResult, msg string) {
	clearScreen()
	PrintSuccess(fmt.Sprintf("=== Page %d ===", pageNum))
	fmt.Println()
	if len(results) == 0 {
		PrintError("  (no results on this page)")
	} else {
		for _, r := range results {
			PrintSuccess(fmt.Sprintf("  %s -> %s", r.Key, r.Value))
		}
	}
	fmt.Println()
	if msg != "" {
		PrintError(msg)
	}
	PrintSuccess("[n] next   [p] prev   [s] stop")
	fmt.Println()
}
