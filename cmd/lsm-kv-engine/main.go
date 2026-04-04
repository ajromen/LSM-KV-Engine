package main

import (
	"fmt"

	"github.com/ajromen/LSM-KV-Engine/internal/cli"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/core"
	"github.com/ajromen/LSM-KV-Engine/internal/flags"
)

func main() {
	err := config.LoadConfig(flags.ParseFlags())
	if err != nil {
		panic(err)
	}
	engine, err := core.NewEngine()
	if err != nil {
		cli.PrintError(fmt.Sprint("Error: ", err))
		return
	}
	cli.RunCli(engine)
}
