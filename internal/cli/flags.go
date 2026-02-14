package cli

import (
	"flag"
	"strconv"
)

type FLags struct {
	ConfigPath          *string
	Debug               *bool
	MemtableMaxSize     *int
	MemtableMaxSizeKb   *int
	MemtableType        *string
	BlockCacheMaxBlocks *int

	BlockSize *int

	WALSegmentSize  *int
	WALSyncInterval *int
	WALMaxSegments  *int
}

func ParseFlags() *FLags {
	var configPathOpt OptionalString
	var debugOpt OptionalBool
	var mtMaxSizeOpt OptionalInt
	var mtMaxSizeKbOpt OptionalInt
	var mtTypeOpt OptionalString
	var blockCacheMaxBlocksOpt OptionalInt

	var blockSizeOpt OptionalInt
	var walSegSizeOpt OptionalInt
	var walSyncOpt OptionalInt
	var walMaxSegOpt OptionalInt

	flagString(&configPathOpt, "config", "Path to config file")
	flagString(&configPathOpt, "c", "Path to config file")

	flagBool(&debugOpt, "debug", "Debug mode")
	flagBool(&debugOpt, "d", "Debug mode")

	flagInt(&mtMaxSizeOpt, "memtable-max-size", "Max memtable size (entries or bytes, depending on implementation)")
	flagInt(&mtMaxSizeKbOpt, "memtable-max-size-kb", "Max memtable size in kilobytes")
	flagString(&mtTypeOpt, "memtable-type", "hashmap, skiplist or btree")

	flagInt(&blockCacheMaxBlocksOpt, "block-cache-max-blocks", "Max number of blocks in the block cache")

	flagInt(&blockSizeOpt, "block-size", "Block size in bytes")

	flagInt(&walSegSizeOpt, "wal-segment-size", "WAL segment size in bytes")
	flagInt(&walSyncOpt, "wal-sync-interval", "WAL sync interval in milliseconds (0=only on commit/flush)")
	flagInt(&walMaxSegOpt, "wal-max-segments", "Max WAL segments to keep (0=unlimited)")

	flag.Parse()

	return &FLags{
		ConfigPath:          configPathOpt.Get(),
		Debug:               debugOpt.Get(),
		MemtableMaxSize:     mtMaxSizeOpt.Get(),
		MemtableMaxSizeKb:   mtMaxSizeKbOpt.Get(),
		MemtableType:        mtTypeOpt.Get(),
		BlockCacheMaxBlocks: blockCacheMaxBlocksOpt.Get(),

		BlockSize:       blockSizeOpt.Get(),
		WALSegmentSize:  walSegSizeOpt.Get(),
		WALSyncInterval: walSyncOpt.Get(),
		WALMaxSegments:  walMaxSegOpt.Get(),
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
