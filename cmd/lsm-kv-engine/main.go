package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ajromen/LSM-KV-Engine/internal/cli"
	"github.com/ajromen/LSM-KV-Engine/internal/core"
)

const helpText = `LSM-KV-Engine CLI

Commands:
  put <key> <value>                               Store a key-value pair
  get <key>                                       Retrieve the value of a key
  del <key>                                       Delete a key
  range_scan <lower> <upper> <pageNum> <pageSize> Range scan with pagination (pageNum 0-based)
  prefix_scan <prefix> <pageNum> <pageSize>       Prefix scan with pagination (pageNum 0-based)
  range_iterate <lower> <upper>                   Start interactive range iterator
  prefix_iterate <prefix>                         Start interactive prefix iterator
  help                                            Show this help message
  clear-all                                       Delete all data
  exit | quit | q                                 Close the engine and exit`

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
	if len(parts) != 3 {
		fmt.Println("Usage: put <key> <value>")
		return
	}
	key := parts[1]
	value := strings.Join(parts[2:], " ")
	if err := engine.Put([]byte(key), []byte(value)); err != nil {
		fmt.Println("Put:", err)
		return
	}
	fmt.Println("Put:", key, " OK")
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
	fmt.Println("Delete:", key, " OK")
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
