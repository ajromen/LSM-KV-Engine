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
    expire <key1> <key2> ... <keyN> <ttl>   Set TTL for a key/keys
    ttl <key>                               Prints remaining TTL for a key (O(1) only if InMemoryTTL=true)
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
		printError(fmt.Sprint("Error: ", err))
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
		case "range_scan":
			handleRangeScan(engine, parts)
		case "prefix_scan":
			handlePrefixScan(engine, parts)
		case "range_iterate":
			handleRangeIterate(engine, parts, reader)
		case "prefix_iterate":
			handlePrefixIterate(engine, parts, reader)
		case "exit", "quit", "q":
			print("Exiting...")
			err := engine.Close()
			if err != nil {
				printError(fmt.Sprint("Closing error: ", err))
				os.Exit(1)
			}
			os.Exit(0)
		case "help":
			printSuccess(fmt.Sprint(helpText))
		case "dataraw":
			handleDataRaw(engine, parts)
		default:
			printError(fmt.Sprint("Unknown command: ", parts[0]))
		}
	}
}

func handlePut(engine *core.Engine, parts []string) {
	if len(parts) < 3 || len(parts) > 4 {
		printError(fmt.Sprint("Usage: put <key> <value> [ttl]"))
		return
	}

	key := parts[1]
	value := parts[2]

	if len(parts) == 4 {
		ttl, err := parseTTL(parts[3])
		if err != nil {
			printError(fmt.Sprint("TTL must be a number"))
			return
		}
		engine.PutWithTTL([]byte(key), []byte(value), ttl)
	} else {
		engine.Put([]byte(key), []byte(value))
	}

	printSuccess(fmt.Sprint("Put: ", key, " OK"))
}
func handleExpire(engine *core.Engine, parts []string) {
	if len(parts) < 3 {
		printError("Usage: expire <key1> <key2> ... <keyN> <ttl>")
		return
	}

	ttl, err := parseTTL(parts[len(parts)-1])
	if err != nil {
		printError("Invalid TTL format")
		return
	}

	keys := parts[1 : len(parts)-1]

	for _, key := range keys {
		val, found, err := engine.Get([]byte(key))
		if err != nil {
			printError(fmt.Sprintf("error checking key '%s': %v", key, err))
			return
		}
		if !found {
			printError(fmt.Sprintf("key not found: %s", key))
			return
		}

		engine.PutWithTTL([]byte(key), val, ttl)
	}

	printSuccess(fmt.Sprintf("Expire set for %d keys (%dms)", len(keys), ttl))
}

func handleGet(engine *core.Engine, parts []string) {
	if len(parts) != 2 {
		printError(fmt.Sprint("Usage: get <key>"))
		return
	}
	key := parts[1]
	value, found, err := engine.Get([]byte(key))
	if err != nil {
		printError(fmt.Sprint("Get:", err))
		return
	}
	if !found {
		printError(fmt.Sprint("Get: key '" + key + "' not found"))
		return
	}
	printSuccess(fmt.Sprint(string(value)))
}

func handleTTL(engine *core.Engine, parts []string) {
	if len(parts) != 2 {
		printError(fmt.Sprint("Usage: ttl <key>"))
		return
	}

	key := parts[1]
	value, found, err := engine.GetTTL([]byte(key))

	if err != nil {
		printError(fmt.Sprint("TTL:", err))
		return
	}
	if !found || value == 0 {
		printError(fmt.Sprint("TTL: key '" + key + "' not found or doesnt have ttl"))
		return
	}
	t := time.UnixMilli(value)
	if t.Before(time.Now()) {
		printError(fmt.Sprint("TTL: key '" + key + "' expired"))
		return
	}
	printSuccess(fmt.Sprintf("Key: '%s', TTL: %dms, Expires At: %s\n", key, t.UnixMilli()-time.Now().UnixMilli(), t.Format("15:04:05 02 Jan 2006 ")))
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
		printError(fmt.Sprint("Usage: delete <key>"))
		return
	}
	key := parts[1]
	engine.Delete([]byte(key))

	printSuccess(fmt.Sprint("Delete: ", key, " OK"))
}

func handleRangeDel(engine *core.Engine, parts []string) {
	if len(parts) != 3 {
		printError(fmt.Sprint("Usage: del-range <key1> <key2>"))
	}
	startKey := parts[1]
	endKey := parts[2]
	engine.RangeDelete([]byte(startKey), []byte(endKey))
	printSuccess(fmt.Sprint("RangeDel: ", startKey, endKey, " OK"))
}

func handleClear(engine *core.Engine, parts []string) {
	if len(parts) != 1 {
		printError(fmt.Sprint("Usage: clear-all"))
		return
	}
	fmt.Print(red + "DELETE ALL DATA? (yes/N): " + reset)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(answer)
	if answer != "yes" {
		printError(fmt.Sprint("Aborted."))
		return
	}
	if err := engine.ClearAll(); err != nil {
		printError(fmt.Sprint("ClearAll:", err))
		return
	}
	printSuccess(fmt.Sprint("ClearAll: OK"))
}

func handleRangeScan(engine *core.Engine, parts []string) {
	if len(parts) != 5 {
		fmt.Println("Usage: range_scan <lower> <upper> <pageNum> <pageSize>")
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
		fmt.Println("Usage: prefix_scan <prefix> <pageNum> <pageSize>")
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

func handleRangeIterate(engine *core.Engine, parts []string, reader *bufio.Reader) {
	if len(parts) != 3 {
		fmt.Println("Usage: range_iterate <lower> <upper>")
		return
	}
	lower := parts[1]
	upper := parts[2]

	it, err := engine.RangeIterate(lower, upper)
	if err != nil {
		fmt.Println("RangeIterate error:", err)
		return
	}

	if !it.Valid() {
		fmt.Println("(no results in range)")
		return
	}

	fmt.Println("Iterator started. Type 'next' for next entry, 'stop' to exit.")
	runIteratorLoop(it, reader)
}

func handlePrefixIterate(engine *core.Engine, parts []string, reader *bufio.Reader) {
	if len(parts) != 2 {
		fmt.Println("Usage: prefix_iterate <prefix>")
		return
	}
	prefix := parts[1]

	it, err := engine.PrefixIterate(prefix)
	if err != nil {
		fmt.Println("PrefixIterate error:", err)
		return
	}

	if !it.Valid() {
		fmt.Println("(no results with prefix)")
		return
	}

	fmt.Println("Iterator started. Type 'next' for next entry, 'stop' to exit.")
	runIteratorLoop(it, reader)
}

func runIteratorLoop(it *core.ActiveIterator, reader *bufio.Reader) {
	for {
		fmt.Print("iter> ")
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)

		switch line {
		case "next":
			result, ok := it.Next()
			if !ok {
				fmt.Println("(end of iterator)")
				return
			}
			fmt.Printf("%s -> %s\n", result.Key, result.Value)
		case "stop":
			fmt.Println("Iterator stopped.")
			return
		default:
			fmt.Println("Unknown iterator command. Use 'next' or 'stop'.")
		}
	}
}

func handleDataRaw(engine *core.Engine, parts []string) {
	if len(parts) != 2 {
		printError(fmt.Sprint("Usage: dataraw <index>"))
		return
	}

	indxStr := parts[1]

	indx, err := strconv.Atoi(indxStr)
	if err != nil {
		printError(fmt.Sprint("invalid index:", indxStr))
		return
	}

	engine.DataRaw(indx)
}

func printError(err string) {
	fmt.Println(red + err + reset)
}

func printSuccess(str string) {
	fmt.Println(green + str + reset)

}
