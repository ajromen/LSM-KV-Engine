package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/ajromen/LSM-KV-Engine/internal/cli"
	"github.com/ajromen/LSM-KV-Engine/internal/core"
)

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
		case "exit", "quit":
			engine.Close()
			return
		case "help":
			fmt.Println("commands...")
		default:
			fmt.Println("Unknown command: ", parts[0])
		}
	}
}

func handlePut(engine *core.Engine, parts []string) {
	if len(parts) < 3 {
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
	if len(parts) < 2 {
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
		fmt.Println("Get: not found")
		return
	}
	fmt.Println(string(value))
}

func handleDelete(engine *core.Engine, parts []string) {
	if len(parts) < 2 {
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
