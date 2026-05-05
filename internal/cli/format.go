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
    help-probabilistic
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
  Snapshots:
    snapshot <key>                    Retain all future versions of a key
    get-versions <key>                List all versions (newest first)
    get-version <key> <version>       Get specific version (0=current, 1=previous, ...)
  Backups:
    list-backups                 List all backups (includes checkpoints)
    create-backup [type]         Create backup of type: full, incremental, checkpoint (if none is provided uses default)
    delete-backup <id>           Delete backup by id
    restore-backup <id>          Delete everything and restore to backup (create-backup recomended)
    cascade-delete-backup <id>   Delete backup and all backups that are dependent on it
    delete-all-backups           Deletes all backups
Notes:
  ttl: time-to-live in seconds,
  unit suffixes: ms, s (default), min, h, D, M, Y`

const helpProbabilistic = `LSM-KV-Engine CLI Help Probabilistic
Probabilistic operations:
  Bloom Filter:
    bf-create <name> <expectedElements> <falsePositiveRate>   Create Bloom Filter
    bf-add <name> <element>                                   Add element
    bf-contains <name> <element>                              Check membership
    bf-delete <name>                                          Delete Bloom Filter
  Count-Min Sketch:
    cms-create <name>         Create Count-Min Sketch
    cms-add <name> <event>    Add event
    cms-freq <name> <event>   Estimate frequency
    cms-delete <name>         Delete Count-Min Sketch
  HyperLogLog:
    hll-create <name>          Create HyperLogLog
    hll-add <name> <element>   Add element
    hll-count <name>           Estimate unique count
    hll-delete <name>          Delete HyperLogLog
  SimHash:
    sh-store <name> <text...>   Store SimHash fingerprint
    sh-dist <name1> <name2>     Compare fingerprints
    sh-delete <name>            Delete SimHash`

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
		PrintSuccess("  print-wal         Print decoded raw data for all wal segments")
	}
}

func printHelpProbabilisticMessage() {
	PrintSuccess(helpProbabilistic)
}
