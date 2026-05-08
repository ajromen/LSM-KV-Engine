package flags

import (
	"flag"
	"strconv"
)

type FLags struct {
	ConfigPath                 *string
	Debug                      *bool
	CreateDefaultConfig        *bool
	MemtableMaxSize            *int
	MemtableMaxSizeB           *uint64
	MemtableType               *string
	Instances                  *int
	SSTableFormat              *string
	BlockCacheMaxBlocks        *int
	LSMCompactionAlgorithm     *string
	TTLInMemoryTTL             *bool
	TTLRefreshRate             *uint64
	TokenBucketMaxTokens       *int64
	TokenBucketResetIntervalMs *int64
	WalMaxBlocks               *int64
}

func ParseFlags() *FLags {
	var configPathOpt OptionalString
	var debugOpt OptionalBool
	var createDefaultConfigOpt OptionalBool
	var mtMaxSizeOpt OptionalInt
	var mtMaxSizeBOpt OptionalUInt64
	var instancesOpt OptionalInt
	var mtTypeOpt OptionalString
	var sstFormatOpt OptionalString
	var BlockCacheMaxBlocksOpt OptionalInt
	var LSMCompactionAlgorithmOpt OptionalString
	var TTLInMemoryTTLOpt OptionalBool
	var TTLRefreshRateOpt OptionalUInt64
	var tokenBucketMaxTokensOpt OptionalInt64
	var tokenBucketResetIntervalMsOpt OptionalInt64
	var walMaxBlocksOpt OptionalInt64

	flagString(&configPathOpt, "config", "Path to config file")
	flagString(&configPathOpt, "c", "Path to config file")

	flagBool(&debugOpt, "debug", "Debug mode")
	flagBool(&debugOpt, "d", "Debug mode")
	flagBool(&createDefaultConfigOpt, "create-default-config", "Creates default config")
	flagInt(&mtMaxSizeOpt, "memtable-max-size", "Max memtable size in number of entries")
	flagUint64(&mtMaxSizeBOpt, "memtable-max-size-b", "Max memtable size in bytes")
	flagInt(&instancesOpt, "instances", "Number of memtable instances")
	flagString(&mtTypeOpt, "memtable-type", "hashmap, skiplist, btree, rbtree, avltree")
	flagString(&sstFormatOpt, "sst-format", "sst format (single-file / multi-file)")
	flagInt(&BlockCacheMaxBlocksOpt, "block-cache-max-blocks", "Max number of blocks in the block cache")
	flagString(&LSMCompactionAlgorithmOpt, "lsm-compaction", "LSM compaction algorithm: size-tiered, leveled")
	flagBool(&TTLInMemoryTTLOpt, "ttl-in-memory", "Keep {Key,TTL} in memory and allow expiry notifications")
	flagUint64(&TTLRefreshRateOpt, "ttl-refresh-rate", "Timer in ms for checking expiring keys (Works only if ttl-in-memory=true")
	flagInt64(&tokenBucketMaxTokensOpt, "token-bucket-max-tokens", "Max tokens in token bucket (0 = disabled)")
	flagInt64(&tokenBucketResetIntervalMsOpt, "token-bucket-reset-ms", "Token bucket reset interval in milliseconds")
	flagInt64(&walMaxBlocksOpt, "wal-seg-max-blocks", "Max blocks in WAL segment")

	flag.Parse()

	return &FLags{
		ConfigPath:                 configPathOpt.Get(),
		Debug:                      debugOpt.Get(),
		CreateDefaultConfig:        createDefaultConfigOpt.Get(),
		MemtableMaxSize:            mtMaxSizeOpt.Get(),
		MemtableMaxSizeB:           mtMaxSizeBOpt.Get(),
		MemtableType:               mtTypeOpt.Get(),
		Instances:                  instancesOpt.Get(),
		SSTableFormat:              sstFormatOpt.Get(),
		LSMCompactionAlgorithm:     LSMCompactionAlgorithmOpt.Get(),
		TTLInMemoryTTL:             TTLInMemoryTTLOpt.Get(),
		TTLRefreshRate:             TTLRefreshRateOpt.Get(),
		TokenBucketMaxTokens:       tokenBucketMaxTokensOpt.Get(),
		TokenBucketResetIntervalMs: tokenBucketResetIntervalMsOpt.Get(),
		WalMaxBlocks:               walMaxBlocksOpt.Get(),
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
