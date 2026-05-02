package cli

import (
	"fmt"

	"github.com/ajromen/LSM-KV-Engine/internal/core"
)

func handleBatchWrite(engine *core.Engine, parts []string) {
	if len(parts) < 3 || len(parts)%2 == 0 {
		PrintError("Usage: batch-write <key1> <val1> [<key2> <val2> ...]")
		return
	}
	pairs := make([]core.KeyValue, 0)
	for i := 1; i < len(parts); i += 2 {
		pairs = append(pairs, core.KeyValue{
			Key:   []byte(parts[i]),
			Value: []byte(parts[i+1]),
		})
	}
	if err := engine.BatchWrite(pairs); err != nil {
		PrintError(fmt.Sprintf("BatchWrite failed (rolled back): %v", err))
		return
	}
	PrintSuccess(fmt.Sprintf("BatchWrite: %d pairs written OK", len(pairs)))
}

func handleBatchDelete(engine *core.Engine, parts []string) {
	if len(parts) < 2 {
		PrintError("Usage: batch-delete <key1> [<key2> ...]")
		return
	}
	keys := make([][]byte, 0, len(parts)-1)
	for _, k := range parts[1:] {
		keys = append(keys, []byte(k))
	}
	if err := engine.BatchDelete(keys); err != nil {
		PrintError(fmt.Sprintf("BatchDelete failed: %v", err))
		return
	}
	PrintSuccess(fmt.Sprintf("BatchDelete: %d keys deleted OK", len(keys)))
}
