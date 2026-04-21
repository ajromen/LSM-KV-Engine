package cli

import (
	"fmt"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
)

const (
	reset = "\033[0m"
	red   = "\033[31m"
	green = "\033[32m"
	blue  = "\033[34m"
)

const helpText = `LSM-KV-Engine CLI
Commands:
  help   Show this help message
  Basic:
    put <key> <value> [ttl]   Store a key-value pair
    get <key>                 Retrieve the value of a key
    del <key>                 Delete a key
    exit | quit | q           Close the engine and exit
  Aditional:
    del-range <key1> <key2>   Delete a range of keys 
    clear-all                 Delete all data
  TTL:
    expire <key1> <key2> ... <keyN> <ttl>   Set TTL for a key/keys
    ttl <key>                               Prints remaining TTL for a key (O(1) only if InMemoryTTL=true)
  Iterate:
    range-iterate <lower> <upper>   Step through keys in [lower, upper] one at a time
    prefix-iterate <prefix>         Step through keys starting with prefix one at a time
  Scan:
    range-scan <lower> <upper> <pageSize>   Interactive paginated scan in [lower, upper]
    prefix-scan <prefix> <pageSize>         Interactive paginated scan with prefix
    Scan commands: next/n, prev/p, stop/s
  Backups:
    list-backups                 List all backups (includes checkpoints)
    create-backup [type]         Create backup of type: full, incremental, checkpoint (if none is provided uses default)
    delete-backup <id>           Delete backup by id
    restore-backup <id>          Delete everything and restore to backup (create-backup recomended)
    cascade-delete-backup <id>   Delete backup and all backups that are dependent on it
Notes:
  ttl: time-to-live in seconds,
  unit suffixes: ms, s (default), min, h, D, M, Y`

const banner = blue + " \n██▓      ██████  ███▄ ▄███▓ ██ ▄█▀██▒   █▓▓█████  ███▄    █   ▄████  ██▓ ███▄    █ ▓█████ \n▓██▒    ▒██    ▒ ▓██▒▀█▀ ██▒ ██▄█▒▓██░   █▒▓█   ▀  ██ ▀█   █  ██▒ ▀█▒▓██▒ ██ ▀█   █ ▓█   ▀ \n▒██░    ░ ▓██▄   ▓██    ▓██░▓███▄░ ▓██  █▒░▒███   ▓██  ▀█ ██▒▒██░▄▄▄░▒██▒▓██  ▀█ ██▒▒███   \n▒██░      ▒   ██▒▒██    ▒██ ▓██ █▄  ▒██ █░░▒▓█  ▄ ▓██▒  ▐▌██▒░▓█  ██▓░██░▓██▒  ▐▌██▒▒▓█  ▄ \n░██████▒▒██████▒▒▒██▒   ░██▒▒██▒ █▄  ▒▀█░  ░▒████▒▒██░   ▓██░░▒▓███▀▒░██░▒██░   ▓██░░▒████▒\n░ ▒░▓  ░▒ ▒▓▒ ▒ ░░ ▒░   ░  ░▒ ▒▒ ▓▒  ░ ▐░  ░░ ▒░ ░░ ▒░   ▒ ▒  ░▒   ▒ ░▓  ░ ▒░   ▒ ▒ ░░ ▒░ ░\n░ ░ ▒  ░░ ░▒  ░ ░░  ░      ░░ ░▒ ▒░  ░ ░░   ░ ░  ░░ ░░   ░ ▒░  ░   ░  ▒ ░░ ░░   ░ ▒░ ░ ░  ░\n  ░ ░   ░  ░  ░  ░      ░   ░ ░░ ░     ░░     ░      ░   ░ ░ ░ ░   ░  ▒ ░   ░   ░ ░    ░   \n    ░  ░      ░         ░   ░  ░        ░     ░            ░       ░  ░           ░    ░   \n" + reset

func PrintError(err string) {
	fmt.Println(red + err + reset)
}

func PrintSuccess(str string) {
	fmt.Println(green + str + reset)

}

func PrintSpecial(str string) {
	fmt.Println(blue + str + reset)

}

func printBanner() {
	fmt.Println(banner)
}

func printReady(usage string) {
	fmt.Print(blue + usage + ">  " + reset)
}

func printHelpMessage() {
	PrintSuccess(helpText)
	if config.GetSettings().Debug {
		PrintSuccess("Debug:\n  dataraw <index>   Print decoded raw data for requested sstable")
	}
}
