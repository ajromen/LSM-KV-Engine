package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ajromen/LSM-KV-Engine/internal/core"
)

func handleTTL(engine *core.Engine, parts []string) {
	if len(parts) != 2 {
		PrintError(fmt.Sprint("Usage: ttl <key>"))
		return
	}

	key := parts[1]
	value, found, err := engine.GetTTL([]byte(key))

	if err != nil {
		PrintError(fmt.Sprint("TTL:", err))
		return
	}
	if !found || value == 0 {
		PrintError(fmt.Sprint("TTL: key '" + key + "' not found or doesnt have ttl"))
		return
	}
	t := time.UnixMilli(value)
	if t.Before(time.Now()) {
		PrintError(fmt.Sprint("TTL: key '" + key + "' expired"))
		return
	}
	PrintSuccess(fmt.Sprintf("Key: '%s', TTL: %dms, Expires At: %s\n", key, t.UnixMilli()-time.Now().UnixMilli(), t.Format("15:04:05 02 Jan 2006 ")))
}

func parseTTL(s string) (int64, error) {
	units := []struct {
		suffix string
		millis int64
	}{
		{"ms", 1},
		{"min", 60_000},
		{"h", 3_600_000},
		{"D", 86_400_000},
		{"M", 2_592_000_000},
		{"Y", 31_536_000_000},
		{"s", 1_000},
	}

	for _, u := range units {
		if strings.HasSuffix(s, u.suffix) {
			n, err := strconv.ParseInt(strings.TrimSuffix(s, u.suffix), 10, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid TTL: %s", s)
			}
			return n * u.millis, nil
		}
	}

	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid TTL: %s", s)
	}
	return n * 1_000, nil
}

func handleExpire(engine *core.Engine, parts []string) {
	if len(parts) < 3 {
		PrintError("Usage: expire <key1> <key2> ... <keyN> <ttl>")
		return
	}

	ttl, err := parseTTL(parts[len(parts)-1])
	if err != nil {
		PrintError("Invalid TTL format")
		return
	}

	keys := parts[1 : len(parts)-1]

	for _, key := range keys {
		val, found, err := engine.Get([]byte(key))
		if err != nil {
			PrintError(fmt.Sprintf("error checking key '%s': %v", key, err))
			return
		}
		if !found {
			PrintError(fmt.Sprintf("key not found: %s", key))
			return
		}

		engine.PutWithTTL([]byte(key), val, ttl)
	}

	PrintSuccess(fmt.Sprintf("Expire set for %d keys (%dms)", len(keys), ttl))
}
