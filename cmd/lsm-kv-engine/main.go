package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	flags := ParseFlags()
	fmt.Println("Config file:", flags.ConfigPath)
	// TODO napraiviti engine.go i poslati mu config i flagove
	RunCli()
}

func RunCli() {
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
			return
		case "help":
			fmt.Println("commands...")
		default:
			fmt.Println("Unknown command: ", parts[0])
		}
	}
}
