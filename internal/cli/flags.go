package cli

import (
	"flag"
	"strconv"
)

// FLags should be pointers to allow nil value when flag is not explicitly provided
type FLags struct {
	ConfigPath        *string
	Debug             *bool
	MemtableMaxSize   *int
	MemtableMaxSizeKb *int
	MemtableType      *string
}

func ParseFlags() *FLags {
	var configPathOpt OptionalString
	var debugOpt OptionalBool
	var mtMaxSizeOpt OptionalInt
	var mtMaxSizeKbOpt OptionalInt
	var mtTypeOpt OptionalString

	flagString(&configPathOpt, "config", "Path to config file")
	flagString(&configPathOpt, "c", "Path to config file")
	flagBool(&debugOpt, "debug", "Debug mode")
	flagBool(&debugOpt, "d", "Debug mode")
	flagInt(&mtMaxSizeOpt, "memtable-max-size", "Max memtable size in bytes")
	flagInt(&mtMaxSizeKbOpt, "memtable-max-size-kb", "Max memtable size in kilobytes")
	flagString(&mtTypeOpt, "memtable-type", "hashmap, skiplist or btree")

	flag.Parse()

	return &FLags{
		ConfigPath:        configPathOpt.Get(),
		Debug:             debugOpt.Get(),
		MemtableMaxSize:   mtMaxSizeOpt.Get(),
		MemtableMaxSizeKb: mtMaxSizeKbOpt.Get(),
		MemtableType:      mtTypeOpt.Get(),
	}
}

func flagString(opt *OptionalString, name, description string) {
	flag.Func(name, description, func(v string) error {
		opt.val = v
		opt.set = true
		return nil
	})
}

func flagInt(opt *OptionalInt, name, description string) {
	flag.Func(name, description, func(v string) error {
		var err error
		opt.val, err = strconv.Atoi(v)
		opt.set = true
		if err != nil {
			opt.set = false
		}
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
