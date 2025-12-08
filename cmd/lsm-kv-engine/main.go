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
		case "exit", "quit":
			return
		case "help":
			fmt.Println("commands...")
		default:
			fmt.Println("Unknown command: ", parts[0])
		}
	}
}
