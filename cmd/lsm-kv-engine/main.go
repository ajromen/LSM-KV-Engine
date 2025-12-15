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

		switch parts[0] {
		case "put":
			//TODO pozivati metode engine-a za put get delete ( za sad )
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
