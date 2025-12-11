package main

import (
	"flag"
	"fmt"
	"os"
)

const DEFAULT_CONFIG_PATH = "configs/default.json"

type CLIFLags struct {
	ConfigPath string
	Debug      bool
}

func ParseFlags() *CLIFLags {
	flags := &CLIFLags{}

	flag.StringVar(&flags.ConfigPath, "config", DEFAULT_CONFIG_PATH, "Path to config file")
	flag.StringVar(&flags.ConfigPath, "c", DEFAULT_CONFIG_PATH, "Path to config file")
	flag.BoolVar(&flags.Debug, "debug", false, "Debug mode")
	flag.BoolVar(&flags.Debug, "d", false, "Debug mode")

	flag.Parse()

	checkFlags(flags)

	return flags
}

func checkFlags(flags *CLIFLags) {
	_, err := os.Stat(flags.ConfigPath)

	if err != nil {
		fmt.Println("Config file not found, using default: ", DEFAULT_CONFIG_PATH)
		flags.ConfigPath = DEFAULT_CONFIG_PATH
	}
}
