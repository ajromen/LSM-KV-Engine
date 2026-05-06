package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ajromen/LSM-KV-Engine/internal/core"
)

func handleBFCreate(e *core.Engine, p []string) {
	if len(p) != 4 {
		PrintError("Usage: bf-create <name> <expectedElements> <falsePositiveRate>")
		return
	}
	n, err := strconv.ParseUint(p[2], 10, 64)
	if err != nil {
		PrintError("expectedElements must be a positive integer")
		return
	}

	fp, err := strconv.ParseFloat(p[3], 64)
	if err != nil {
		PrintError("falsePositiveRate must be a number")
		return
	}
	if err := e.BFCreate(p[1], uint(n), fp); err != nil {
		PrintError(fmt.Sprint("bf-create:", err))
		return
	}
	PrintSuccess(fmt.Sprintf("BloomFilter '%s' created", p[1]))
}

func handleBFAdd(e *core.Engine, p []string) {
	if len(p) != 3 {
		PrintError("Usage: bf-add <name> <element>")
		return
	}
	if err := e.BFAdd(p[1], []byte(p[2])); err != nil {

		PrintError(fmt.Sprint("bf-add:", err))
		return

	}
	PrintSuccess(fmt.Sprintf("Added '%s' to BloomFilter '%s'", p[2], p[1]))
}

func handleBFContains(e *core.Engine, p []string) {
	if len(p) != 3 {
		PrintError("Usage: bf-contains <name> <element>")
		return
	}
	ok, err := e.BFContains(p[1], []byte(p[2]))
	if err != nil {
		PrintError(fmt.Sprint("bf-contains:", err))
		return
	}
	if ok {
		PrintSuccess(fmt.Sprintf("'%s' might be in BloomFilter '%s'", p[2], p[1]))
	} else {
		PrintError(fmt.Sprintf("'%s' is definitely NOT in BloomFilter '%s'", p[2], p[1]))
	}
}

func handleBFDelete(e *core.Engine, p []string) {
	if len(p) != 2 {
		PrintError("Usage: bf-delete <name>")
		return
	}
	e.BFDelete(p[1])
	PrintSuccess(fmt.Sprintf("BloomFilter '%s' deleted", p[1]))
}

func handleCMSCreate(e *core.Engine, p []string) {
	if len(p) != 2 {
		PrintError("Usage: cms-create <name>")
		return
	}
	if err := e.CMSCreate(p[1]); err != nil {
		PrintError(fmt.Sprint("cms-create:", err))
		return
	}
	PrintSuccess(fmt.Sprintf("CountMinSketch '%s' created", p[1]))
}

func handleCMSAdd(e *core.Engine, p []string) {
	if len(p) != 3 {
		PrintError("Usage: cms-add <name> <event>")
		return
	}
	if err := e.CMSAdd(p[1], []byte(p[2])); err != nil {

		PrintError(fmt.Sprint("cms-add:", err))
		return
	}
	PrintSuccess(fmt.Sprintf("Added '%s' to CountMinSketch '%s'", p[2], p[1]))
}

func handleCMSFreq(e *core.Engine, p []string) {
	if len(p) != 3 {
		PrintError("Usage: cms-freq <name> <event>")
		return
	}
	freq, err := e.CMSFrequency(p[1], []byte(p[2]))
	if err != nil {
		PrintError(fmt.Sprint("cms-freq:", err))
		return
	}
	PrintSuccess(fmt.Sprintf("Estimated frequency of '%s' in '%s': %d", p[2], p[1], freq))
}

func handleCMSDelete(e *core.Engine, p []string) {
	if len(p) != 2 {
		PrintError("Usage: cms-delete <name>")
		return
	}
	e.CMSDelete(p[1])
	PrintSuccess(fmt.Sprintf("CountMinSketch '%s' deleted", p[1]))
}

func handleHLLCreate(e *core.Engine, p []string) {
	if len(p) != 2 {
		PrintError("Usage: hll-create <name>")
		return
	}
	if err := e.HLLCreate(p[1]); err != nil {
		PrintError(fmt.Sprint("hll-create:", err))
		return
	}
	PrintSuccess(fmt.Sprintf("HyperLogLog '%s' created", p[1]))
}

func handleHLLAdd(e *core.Engine, p []string) {
	if len(p) != 3 {
		PrintError("Usage: hll-add <name> <element>")
		return
	}
	if err := e.HLLAdd(p[1], []byte(p[2])); err != nil {

		PrintError(fmt.Sprint("hll-add:", err))
		return

	}
	PrintSuccess(fmt.Sprintf("Added '%s' to HyperLogLog '%s'", p[2], p[1]))
}

func handleHLLCount(e *core.Engine, p []string) {
	if len(p) != 2 {
		PrintError("Usage: hll-count <name>")
		return
	}
	n, err := e.HLLCardinality(p[1])
	if err != nil {
		PrintError(fmt.Sprint("hll-count:", err))
		return
	}
	PrintSuccess(fmt.Sprintf("Estimated cardinality of '%s': %d", p[1], n))
}

func handleHLLDelete(e *core.Engine, p []string) {
	if len(p) != 2 {
		PrintError("Usage: hll-delete <name>")
		return
	}
	e.HLLDelete(p[1])
	PrintSuccess(fmt.Sprintf("HyperLogLog '%s' deleted", p[1]))
}

func handleSHStore(e *core.Engine, p []string) {
	if len(p) < 3 {
		PrintError("Usage: sh-store <name> <text...>")
		return
	}
	text := strings.Join(p[2:], " ")
	if err := e.SimHashStore(p[1], text); err != nil {
		PrintError(fmt.Sprint("sh-store:", err))
		return
	}
	PrintSuccess(fmt.Sprintf("SimHash for '%s' stored", p[1]))
}

func handleSHDist(e *core.Engine, p []string) {
	if len(p) != 3 {
		PrintError("Usage: sh-dist <name1> <name2>")
		return
	}
	d, err := e.SimHashDistance(p[1], p[2])
	if err != nil {
		PrintError(fmt.Sprint("sh-dist:", err))
		return
	}
	PrintSuccess(fmt.Sprintf("Hamming distance between '%s' and '%s': %d", p[1], p[2], d))
}

func handleSHDelete(e *core.Engine, p []string) {
	if len(p) != 2 {
		PrintError("Usage: sh-delete <name>")
		return
	}
	e.SimHashDelete(p[1])
	PrintSuccess(fmt.Sprintf("SimHash '%s' deleted", p[1]))
}
