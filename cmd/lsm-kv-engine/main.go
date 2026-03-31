package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ajromen/LSM-KV-Engine/internal/cli"
	"github.com/ajromen/LSM-KV-Engine/internal/core"
)

const helpText = `LSM-KV-Engine CLI
Commands:
  put <key> <value> [ttl]   Store a key-value pair
  get <key>                 Retrieve the value of a key
  del <key>                 Delete a key
  expire <key> <ttl>        Set TTL for a key
  ttl <key>                 Prints remaining TTL for a key (O(1) only if InMemoryTTL=true)
  help                      Show this help message
  clear-all                 Delete all data
  exit | quit | q           Close the engine and exit
Notes:
  ttl: time-to-live in seconds`

func main() {
	flags := cli.ParseFlags()
	engine, err := core.NewEngine(flags)
	if err != nil {
		fmt.Println("Greska: ", err)
		return
	}
	RunCli(engine)
}

func RunCli(engine *core.Engine) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("> ")

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
		ttl, err := strconv.ParseInt(parts[3], 10, 64)
		if err != nil {
			fmt.Println("TTL must be a number")
			return
		}
		if err := engine.PutWithTTL([]byte(key), []byte(value), ttl); err != nil {
			fmt.Println("Put:", err)
			return
		}
	} else {
		if err := engine.Put([]byte(key), []byte(value)); err != nil {
			fmt.Println("Put:", err)
			return
		}
	}

	fmt.Println("Put:", key, "OK")
}

func handleExpire(engine *core.Engine, parts []string) {
	if len(parts) < 3 {
		fmt.Println("Usage: expire <key> <ttl>")
		return
	}
	key := parts[1]
	ttl, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		fmt.Println("TTL must be a number")
		return
	}
	err = engine.PutWithTTL([]byte(key), nil, ttl)
	if err != nil {
		fmt.Println("EXPIRE:", err)
		return
	}
	fmt.Printf("Expire: '%s', %ds OK\n", key, ttl)
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
		fmt.Println("TTL: key '" + key + "' not found")
		return
	}
	t := time.Unix(value, 0)
	fmt.Printf("Key: '%s', TTL: %ds, Expires At: %s\n", key, t.Unix()-time.Now().Unix(), t.Format("15:04:05 02 Jan 2006 "))
}

func handleDelete(engine *core.Engine, parts []string) {
	if len(parts) != 2 {
		fmt.Println("Usage: delete <key>")
		return
	}
	key := parts[1]
	if err := engine.Delete([]byte(key)); err != nil {
		fmt.Println("Delete:", err)
		return
	}

	fmt.Println("Delete:", key, "OK")
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
