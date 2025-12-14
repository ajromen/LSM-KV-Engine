package cli

import (
	"flag"
)

// FLags should be pointers to allow nil value when flag is not explicitly provided
type FLags struct {
	ConfigPath *string
	Debug      *bool
}

func ParseFlags() *FLags {
	var configPathOpt OptionalString
	var debugOpt OptionalBool

	flagString(&configPathOpt, "config", "Path to config file")
	flagString(&configPathOpt, "c", "Path to config file")
	flagBool(&debugOpt, "debug", "Debug mode")
	flagBool(&debugOpt, "d", "Debug mode")

	flag.Parse()

	return &FLags{
		ConfigPath: configPathOpt.Get(),
		Debug:      debugOpt.Get(),
	}
}

func flagString(opt *OptionalString, name, description string) {
	flag.Func(name, description, func(v string) error {
		opt.val = v
		opt.set = true
		return nil
	})
}

func flagBool(opt *OptionalBool, name, description string) {
	flag.Func(name, description, func(v string) error {
		opt.val = v == "true"
		opt.set = true
		return nil
	})
}
