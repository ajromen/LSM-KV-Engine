package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ajromen/LSM-KV-Engine/internal/cli"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/core"
)

const helpText = `LSM-KV-Engine CLI
Commands:
  help   Show this help message
  Basic:
    put <key> <value> [ttl]   Store a key-value pair
    get <key>                 Retrieve the value of a key
    del <key>                 Delete a key
    exit | quit | q           Close the engine and exit
  TTL:
    expire <key> <ttl>   Set TTL for a key
    ttl <key>            Prints remaining TTL for a key (O(1) only if InMemoryTTL=true)
  Aditional:
    del-range <key1> <key2>   Delete a range of keys 
    clear-all                 Delete all data
Notes:
  ttl: time-to-live in seconds,
  unit suffixes: ms, s (default), min, h, D, M, Y`

const (
	reset = "\033[0m"
	red   = "\033[31m"
	green = "\033[32m"
	blue  = "\033[34m"
)

func main() {
	flags := cli.ParseFlags()
	err := config.LoadConfig(flags)
	if err != nil {
		panic(err)
	}
	engine, err := core.NewEngine()
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	RunCli(engine)
}

func RunCli(engine *core.Engine) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println(blue + " \n██▓      ██████  ███▄ ▄███▓ ██ ▄█▀██▒   █▓▓█████  ███▄    █   ▄████  ██▓ ███▄    █ ▓█████ \n▓██▒    ▒██    ▒ ▓██▒▀█▀ ██▒ ██▄█▒▓██░   █▒▓█   ▀  ██ ▀█   █  ██▒ ▀█▒▓██▒ ██ ▀█   █ ▓█   ▀ \n▒██░    ░ ▓██▄   ▓██    ▓██░▓███▄░ ▓██  █▒░▒███   ▓██  ▀█ ██▒▒██░▄▄▄░▒██▒▓██  ▀█ ██▒▒███   \n▒██░      ▒   ██▒▒██    ▒██ ▓██ █▄  ▒██ █░░▒▓█  ▄ ▓██▒  ▐▌██▒░▓█  ██▓░██░▓██▒  ▐▌██▒▒▓█  ▄ \n░██████▒▒██████▒▒▒██▒   ░██▒▒██▒ █▄  ▒▀█░  ░▒████▒▒██░   ▓██░░▒▓███▀▒░██░▒██░   ▓██░░▒████▒\n░ ▒░▓  ░▒ ▒▓▒ ▒ ░░ ▒░   ░  ░▒ ▒▒ ▓▒  ░ ▐░  ░░ ▒░ ░░ ▒░   ▒ ▒  ░▒   ▒ ░▓  ░ ▒░   ▒ ▒ ░░ ▒░ ░\n░ ░ ▒  ░░ ░▒  ░ ░░  ░      ░░ ░▒ ▒░  ░ ░░   ░ ░  ░░ ░░   ░ ▒░  ░   ░  ▒ ░░ ░░   ░ ▒░ ░ ░  ░\n  ░ ░   ░  ░  ░  ░      ░   ░ ░░ ░     ░░     ░      ░   ░ ░ ░ ░   ░  ▒ ░   ░   ░ ░    ░   \n    ░  ░      ░         ░   ░  ░        ░     ░            ░       ░  ░           ░    ░   \n" + reset)
	for {
		fmt.Print(blue + ">  " + reset)

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
		case "exit", "quit", "q":
			print("Exiting...")
			err := engine.Close()
			if err != nil {
				fmt.Println("Closing error: ", err)
				os.Exit(1)
			}
			os.Exit(0)
		case "help":
			fmt.Println(helpText)
		case "dataraw":
			handleDataRaw(engine, parts)
		default:
			fmt.Println("Unknown command: ", parts[0])
		}
	}
}

func handlePut(engine *core.Engine, parts []string) {
	if len(parts) < 3 || len(parts) > 4 {
		fmt.Println("Usage: put <key> <value> [ttl]")
		return
	}

	key := parts[1]
	value := parts[2]

	if len(parts) == 4 {
		ttl, err := parseTTL(parts[3])
		if err != nil {
			fmt.Println("TTL must be a number")
			return
		}
		engine.PutWithTTL([]byte(key), []byte(value), ttl)
	} else {
		engine.Put([]byte(key), []byte(value))
	}

	fmt.Println("Put:", key, "OK")
}

func handleExpire(engine *core.Engine, parts []string) {
	if len(parts) < 3 {
		fmt.Println("Usage: expire <key> <ttl>")
		return
	}
	key := parts[1]
	ttl, err := parseTTL(parts[2])
	if err != nil {
		fmt.Println("TTL must be a number")
		return
	}
	engine.PutWithTTL([]byte(key), nil, ttl)
	fmt.Printf("Expire: '%s', %dms OK\n", key, ttl)
}

func handleGet(engine *core.Engine, parts []string) {
	if len(parts) != 2 {
		fmt.Println("Usage: get <key>")
		return
	}
	key := parts[1]
	value, found, err := engine.Get([]byte(key))
	if err != nil {
		fmt.Println("Get:", err)
		return
	}
	if !found {
		fmt.Println("Get: key '" + key + "' not found")
		return
	}
	fmt.Println(string(value))
}

func handleTTL(engine *core.Engine, parts []string) {
	if len(parts) != 2 {
		fmt.Println("Usage: ttl <key>")
		return
	}

	key := parts[1]
	value, found, err := engine.GetTTL([]byte(key))
	if err != nil {
		fmt.Println("TTL:", err)
		return
	}
	if !found {
		fmt.Println("TTL: key '" + key + "' not found or doesnt have ttl")
		return
	}
	t := time.UnixMilli(value)
	if t.Before(time.Now()) {
		fmt.Println("TTL: key '" + key + "' expired")
		return
	}
	fmt.Printf("Key: '%s', TTL: %dms, Expires At: %s\n", key, t.UnixMilli()-time.Now().UnixMilli(), t.Format("15:04:05 02 Jan 2006 "))
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

func handleDelete(engine *core.Engine, parts []string) {
	if len(parts) != 2 {
		fmt.Println("Usage: delete <key>")
		return
	}
	key := parts[1]
	engine.Delete([]byte(key))

	fmt.Println("Delete:", key, "OK")
}

func handleRangeDel(engine *core.Engine, parts []string) {
	if len(parts) != 3 {
		fmt.Println("Usage: del-range <key1> <key2>")
	}
	startKey := parts[1]
	endKey := parts[2]
	engine.RangeDelete([]byte(startKey), []byte(endKey))
	fmt.Println("RangeDel:", startKey, endKey, "OK")
}

func handleClear(engine *core.Engine, parts []string) {
	if len(parts) != 1 {
		fmt.Println("Usage: clear-all")
		return
	}
	fmt.Print("DELETE ALL DATA? (yes/N): ")
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(answer)
	if answer != "yes" {
		fmt.Println("Aborted.")
		return
	}
	if err := engine.ClearAll(); err != nil {
		fmt.Println("ClearAll:", err)
		return
	}
	fmt.Println("ClearAll: OK")
}

func handleDataRaw(engine *core.Engine, parts []string) {
	if len(parts) != 2 {
		fmt.Println("Usage: dataraw <index>")
		return
	}

	indxStr := parts[1]

	indx, err := strconv.Atoi(indxStr)
	if err != nil {
		fmt.Println("invalid index:", indxStr)
		return
	}

	engine.DataRaw(indx)
}
