package cli

import (
	"flag"
	"strconv"
)

// FLags should be pointers to allow nil value when flag is not explicitly provided
type FLags struct {
	ConfigPath             *string
	Debug                  *bool
	MemtableMaxSize        *int
	MemtableMaxSizeB       *uint64
	MemtableType           *string
	Instances              *int
	SSTableFormat          *string
	BlockCacheMaxBlocks    *int
	LSMCompactionAlgorithm *string
	TTLInMemoryTTL         *bool
}

func ParseFlags() *FLags {
	var configPathOpt OptionalString
	var debugOpt OptionalBool
	var mtMaxSizeOpt OptionalInt
	var mtMaxSizeBOpt OptionalUInt64
	var instancesOpt OptionalInt
	var mtTypeOpt OptionalString
	var sstFormatOpt OptionalString
	var BlockCacheMaxBlocksOpt OptionalInt
	var LSMCompactionAlgorithmOpt OptionalString
	var TTLInMemoryTTLOpt OptionalBool

	flagString(&configPathOpt, "config", "Path to config file")
	flagString(&configPathOpt, "c", "Path to config file")
	flagBool(&debugOpt, "debug", "Debug mode")
	flagBool(&debugOpt, "d", "Debug mode")
	flagInt(&mtMaxSizeOpt, "memtable-max-size", "Max memtable size in number of entries")
	flagUint64(&mtMaxSizeBOpt, "memtable-max-size-b", "Max memtable size in bytes")
	flagInt(&instancesOpt, "instances", "Number of memtable instances")
	flagString(&mtTypeOpt, "memtable-type", "hashmap, skiplist, btree, rbtree, avltree")
	flagString(&sstFormatOpt, "sst-format", "sst format (single-file / multi-file)")
	flagInt(&BlockCacheMaxBlocksOpt, "block-cache-max-blocks", "Max number of blocks in the block cache")
	flagString(&LSMCompactionAlgorithmOpt, "lsm-compaction", "LSM compaction algorithm: size-tiered, leveled")
	flagBool(&TTLInMemoryTTLOpt, "ttl-in-memory", "Keep {Key,TTL} in memory and allow expiry notifications")

	flag.Parse()

	return &FLags{
		ConfigPath:             configPathOpt.Get(),
		Debug:                  debugOpt.Get(),
		MemtableMaxSize:        mtMaxSizeOpt.Get(),
		MemtableMaxSizeB:       mtMaxSizeBOpt.Get(),
		MemtableType:           mtTypeOpt.Get(),
		Instances:              instancesOpt.Get(),
		SSTableFormat:          sstFormatOpt.Get(),
		LSMCompactionAlgorithm: LSMCompactionAlgorithmOpt.Get(),
		TTLInMemoryTTL:         TTLInMemoryTTLOpt.Get(),
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

func flagUint16(opt *OptionalUInt16, name, description string) {
	flag.Func(name, description, func(v string) error {
		val, err := strconv.ParseUint(v, 10, 16)
		if err != nil {
			opt.set = false
			return err
		}

		opt.val = uint16(val)
		opt.set = true
		return nil
	})
}

func flagUint32(opt *OptionalUInt32, name, description string) {
	flag.Func(name, description, func(v string) error {
		val, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			opt.set = false
			return err
		}

		opt.val = uint32(val)
		opt.set = true
		return nil
	})
}

func flagUint64(opt *OptionalUInt64, name, description string) {
	flag.Func(name, description, func(v string) error {
		val, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			opt.set = false
			return err
		}

		opt.val = uint64(val)
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
