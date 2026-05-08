# LSM-KV-Engine

Log-Structured Merge-Tree Key-Value Storage Engine written in GoLang.

---

## Table of Contents

- [Features](#features)
- [Getting Started](#getting-started)
- [Configuration](#configuration)
- [Save Directory](#save-directory)
- [CLI Reference](#cli-reference)
- [PREFIX_ITERATE / RANGE_ITERATE](#prefix_iterate--range_iterate)
- [SSTable Files](#sstable-files)

---

## **Features**

| **Feature**              | **Description**                                                        |  
|--------------------------|------------------------------------------------------------------------|  
| LSM-Tree core            | Memtable → L0 SSTables → leveled compaction                            |   
| Compaction strategies    | LeveledCompaction and SizeTieredCompaction via Strategy pattern        |   
| TTL support              | Per-key TTL as int64 Unix ms; min-heap janitor with wakeCh interrupt   |   
| SeqId versioning         | Monotonic sequence IDs; merge iterator resolves ties by SeqId DESC     |   
| Bloom filters            | Per-SSTable probabilistic membership check to skip disk reads          |   
| Block cache              | LRU block cache with mutex; invalidated atomically after compaction    |   
| Manifest                 | Compaction metadata persisted via write-then-rename for crash safety   |   
| Range & prefix scans     | Paginated RangeScan/PrefixScan and stateful iterator variants          |   
| Range deletes            | Efficient range tombstone support                                      |   
| Pub/Sub notifier         | Subscribe to key-range changes via buffered Listener channels          |   
| Rate limiting            | Token bucket for write throttling                                      |   
| Snapshots                | Retain and retrieve historical versions of a key                       |   
| Backups                  | Full, incremental, and checkpoint backup/restore support               |   
| Probabilistic structures | Bloom Filter, Count-Min Sketch, HyperLogLog, SimHash                   |   
| CLI                      | Interactive CLI with tab-autocomplete and command history via readline |  

---

## Getting Started

### Prerequisites

- Go 1.21+

### Build

```bash
git clone https://github.com/ajromen/LSM-KV-Engine
cd LSM-KV-Engine
go build ./...
```

### Run CLI

```bash
go run ./cli/lsm-kv-engine/main.go
```

### Embed as a Library

```go
import "github.com/ajromen/LSM-KV-Engine/core"

engine, err := core.NewEngine()
if err != nil {
    log.Fatal(err)
}
defer engine.Close()

engine.Put([]byte("hello"), []byte("world"))

val, found, err := engine.Get([]byte("hello"))
```

### Testing

```bash
# All tests
go test ./...

# Specific package with verbose output
go test -v ./internal/lsm/compaction/...
```

---

## **Configuration**

Generate a default config file:  
`go run ./cmd/lsm-kv-engine -create-default-config=true`

Use any other config file:  
`go run ./cmd/lsm-kv-engine -config=/path/to/config.json`

### **Default Paths**

| **OS**  | **Config**                          | **Data**                                    |   
|---------|-------------------------------------|---------------------------------------------|  
| Linux   | ~/.config/lsm-kv-engine             | ~/.local/share/lsm-kv-engine                |   
| macOS   | ~/Library/Preferences/lsm-kv-engine | ~/Library/Application Support/lsm-kv-engine |   
| Windows | %APPDATA%\lsm-kv-engine             | %ProgramData%\lsm-kv-engine                 |   

---

## Save Directory

```
share/lsm-kv-engine/          # Default data directory (platform-specific, see Configuration)
├── SST_MANIFEST.json         # Tracks which SSTables exist at each level 
├── L0_000000.sst             # SSTable files; named by level and sequence number
├── wal/                      # Write-Ahead Log directory
│   ├── WAL_MANIFEST.json     # Tracks active WAL segments
│   └── wal_000001.log        # WAL segment; replayed on crash recovery
└── backups/                  # Backup storage directory
    ├── BACKUP_MANIFEST.json  # Tracks all backups and their dependencies
    └── 177832104/            # Backup 177832104 save directory
```

---

## **CLI Reference**

### **CLI Flags**

Pass flags when launching the engine:

| **Flag**                   | **Description**                                                         |  
|----------------------------|-------------------------------------------------------------------------|  
| `-c`, `-config`            | Path to config file                                                     |   
| `-create-default-config`   | Generate a default config file at the config path                       |   
| `-sst-format`              | SSTable format: single-file or multi-file                               |   
| `-memtable-type`           | Memtable implementation: hashmap, skiplist, btree, rbtree, avltree      |   
| `-memtable-max-size`       | Max memtable size in number of entries                                  |   
| `-memtable-max-size-b`     | Max memtable size in bytes                                              |   
| `-instances`               | Number of memtable instances                                            |   
| `-lsm-compaction`          | Compaction algorithm: size-tiered or leveled                            |   
| `-block-cache-max-blocks`  | Max number of blocks in the block cache                                 |   
| `-ttl-in-memory`           | Keep TTL in memory; enables expiry notifications (true/false)           |   
| `-ttl-refresh-rate`        | Interval in ms for checking expiring keys (requires ttl-in-memory=true) |   
| `-token-bucket-max-tokens` | Max write tokens (0 = unlimited)                                        |   
| `-token-bucket-reset-ms`   | Token bucket reset interval in milliseconds                             |   
| `-wal-seg-max-blocks`      | Max blocks per WAL segment                                              |   
| `-d`, `-debug`             | Enable debug mode                                                       |

example: `go run ./cmd/lsm-kv-engine/main.go -sst-format=single-file -memtable-max-size=4 -d=true`

### **Basic**

|                               |                                                            |
|-------------------------------|------------------------------------------------------------|  
| put `<key>` `<value>` `[ttl]` | Store a key-value pair. ttl is optional (see TTL section). |   
| get `<key>`                   | Retrieve the value for a key.                              |   
| del `<key>`                   | Delete a key (writes a tombstone).                         |   
| help                          | Show the full help message.                                |   
| exit                          | quit                                                       | q | Close the engine and exit the CLI. |   

### **Additional**

|                           |                                                      |
|---------------------------|------------------------------------------------------|  
| del-range `<key1> <key2>` | Delete all keys in the range [key1, key2] inclusive. |   
| clear-all                 | Delete all data in the engine.                       |   
| help-probabilistic        | Show help for probabilistic data structure commands. |   

### **TTL**

|                                    |                                                                  |  
|------------------------------------|------------------------------------------------------------------|  
| expire `<k1>`  ...  `<kN>` `<ttl>` | Set TTL on one or more keys. Last argument is the TTL value.     |   
| ttl `<key>`                        | Print remaining TTL for a key. O(1) only if -ttl-in-memory=true. |   

*TTL unit suffixes: ms, s (default), min, h, D, M, Y — e.g. `expire k 30s` or `put k v 2min`*

### **Iterate**

Stateful iterators step through matching keys one at a time. Use next/n and stop/s to navigate.

|                                   |                                                    |  
|-----------------------------------|----------------------------------------------------|  
| range-iterate `<lower>` `<upper>` | Step through keys in [lower, upper] one at a time. |   
| prefix-iterate `<prefix>`         | Step through keys starting with the given prefix.  |

### **Scan**

Paginated scans load results in pages. Navigate with: next/n, prev/p, stop/s

|                                             |                                                 |  
|---------------------------------------------|-------------------------------------------------|  
| range-scan `<lower>` `<upper>` `<pageSize>` | Interactive paginated scan over [lower, upper]. |   
| prefix-scan `<prefix>` `<pageSize>`         | Interactive paginated scan matching a prefix.   |

### **Snapshots**

Snapshots preserve all future writes to a key, making historical versions retrievable.

|                             |                                                              |  
|-----------------------------|--------------------------------------------------------------|  
| snapshot `<key>`            | Start retaining all future versions of a key.                |   
| get-versions `<key>`        | List all retained versions, newest first.                    |   
| get-version `<key>` `<ver>` | Retrieve a specific version. 0 = current, 1 = previous, etc. |

### **Backups**

Backups capture a consistent point-in-time snapshot of engine state and can be restored at any time.

|                              |                                                                            |  
|------------------------------|----------------------------------------------------------------------------|  
| list-backups                 | List all backups including checkpoints.                                    |   
| create-backup `[type]`       | Create a backup. Types: full, incremental, checkpoint. Default if omitted. |   
| delete-backup `<id>`         | Delete a specific backup by ID.                                            |   
| restore-backup `<id>`        | Wipe all current data and restore from the given backup.                   |   
| cascade-delete-backup `<id>` | Delete a backup and all backups that depend on it.                         |   
| delete-all-backups           | Delete every backup.                                                       |     

*Note: restore-backup backups in case of failure than deletes the backup.*

## **Probabilistic Structures**

Access via the CLI after running help-probabilistic. Each structure is identified by a user-defined name.

### **Bloom Filter**

Probabilistic set membership. Answers 'definitely not in set' or 'probably in set'.

|                                  |                                                                                          |  
|----------------------------------|------------------------------------------------------------------------------------------|  
| bf-create `<name>` `<n>` `<fpr>` | Create a Bloom Filter with expected n elements and false-positive rate fpr (e.g.  0.01). |
| bf-add `<name>` `<element>`      | Add an element.                                                                          |   
| bf-contains `<name>` `<elem>`    | Check membership.                                                                        |   
| bf-delete `<name>`               | Delete the filter.                                                                       |

### **Count-Min Sketch**

Approximate frequency counter for a stream of events.

|                             |                                  |  
|-----------------------------|----------------------------------|  
| cms-create `<name>`         | Create a Count-Min Sketch.       |   
| cms-add `<name>` `<event>`  | Record an occurrence of event.   |   
| cms-freq `<name>` `<event>` | Estimate the frequency of event. |   
| cms-delete `<name>`         | Delete the sketch.               |

### **HyperLogLog**

Approximate distinct-element counter with very low memory usage.

|                              |                                |  
|------------------------------|--------------------------------|  
| hll-create `<name>`          | Create a HyperLogLog.          |   
| hll-add `<name>` `<element>` | Add an element.                |   
| hll-count `<name>`           | Estimate unique element count. |   
| hll-delete `<name>`          | Delete the HyperLogLog.        |

### **SimHash**

Locality-sensitive fingerprinting for near-duplicate text detection.

|                               |                                                 |  
|-------------------------------|-------------------------------------------------|  
| sh-store `<name>` `<text...>` | Store a SimHash fingerprint for the given text. |   
| sh-dist `<name1>` `<name2>`   | Compare two fingerprints (Hamming distance).    |   
| sh-delete `<name>`            | Delete a stored fingerprint.                    |   

---

## **PREFIX_ITERATE / RANGE_ITERATE**

Both operations use `DBIterator` which merges two sorted streams, memtable and SSTable, and returns records sorted
ascending by key

`DBIterator` at each step picks a winner between the two iterators: it compares keys, and for equal keys takes the
higher `SequenceID` . Older versions of the same key and tombstone records are skipped. The memtable side
internally uses a **min-heap** that merges the active and all immutable memtable instances. The SSTable side does the
same a **min-heap** across all SSTable files with a local block cache map for already-loaded blocks and a
Summary→Index→Data hierarchy for efficient seeks. A **hash map** is used to track snapshotted keys.

Prefix or range filtering is applied at a higher level: `Seek` positions the iterator at the first matching key, and
each `Next` checks whether the current key is still within bounds, once it exits, iteration stops.

---

## SSTable Files

| File          | Contents                                                                          |
|---------------|-----------------------------------------------------------------------------------|
| `.data`       | Raw key-value data blocks, block-aligned and optionally prefix-compressed         |
| `.index`      | Sparse index mapping the first key of each data block to its block offset         |
| `.summary`    | Higher-level index over the index blocks; used for fast seek (Summary→Index→Data) |
| `.filter`     | Bloom filter bit array; used to skip disk reads for keys not in this SSTable      |
| `.footer`     | Offsets to all other segments; first thing read when opening an SSTable           |
| `.metadata`   | SSTable metadata: number of blocks, restart interval, sequence ID range, etc.     |
| `.merkle`     | Merkle tree root hash; used for integrity verification of data blocks             |
| `.dictionary` | Compression dictionary for prefix/delta encoding of keys within blocks            |
