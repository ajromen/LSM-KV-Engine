package cli

import (
	"fmt"
	"strconv"

	"github.com/ajromen/LSM-KV-Engine/internal/core"
)

func handleRangeScan(engine *core.Engine, parts []string) {
	if len(parts) != 5 {
		fmt.Println("Usage: range-scan <lower> <upper> <pageNum> <pageSize>")
		return
	}
	lower := parts[1]
	upper := parts[2]
	pageNum, err := strconv.Atoi(parts[3])
	if err != nil {
		fmt.Println("Invalid pageNum:", parts[3])
		return
	}
	pageSize, err := strconv.Atoi(parts[4])
	if err != nil {
		fmt.Println("Invalid pageSize:", parts[4])
		return
	}

	results, err := engine.RangeScan(lower, upper, pageNum, pageSize)
	if err != nil {
		fmt.Println("RangeScan error:", err)
		return
	}
	if len(results) == 0 {
		fmt.Println("(no results)")
		return
	}
	for _, r := range results {
		fmt.Printf("%s -> %s\n", r.Key, r.Value)
	}
}

func handlePrefixScan(engine *core.Engine, parts []string) {
	if len(parts) != 4 {
		fmt.Println("Usage: prefix-scan <prefix> <pageNum> <pageSize>")
		return
	}
	prefix := parts[1]
	pageNum, err := strconv.Atoi(parts[2])
	if err != nil {
		fmt.Println("Invalid pageNum:", parts[2])
		return
	}
	pageSize, err := strconv.Atoi(parts[3])
	if err != nil {
		fmt.Println("Invalid pageSize:", parts[3])
		return
	}

	results, err := engine.PrefixScan(prefix, pageNum, pageSize)
	if err != nil {
		fmt.Println("PrefixScan error:", err)
		return
	}
	if len(results) == 0 {
		fmt.Println("(no results)")
		return
	}
	for _, r := range results {
		fmt.Printf("%s -> %s\n", r.Key, r.Value)
	}
}
