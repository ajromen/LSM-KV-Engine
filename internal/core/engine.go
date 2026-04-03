package core

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/iterator"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
	"github.com/ajromen/LSM-KV-Engine/internal/lsm"
	"github.com/ajromen/LSM-KV-Engine/internal/sequence"
	"github.com/ajromen/LSM-KV-Engine/internal/shared"
	"github.com/ajromen/LSM-KV-Engine/internal/sstable"
	"github.com/ajromen/LSM-KV-Engine/internal/ttl"
)

type Engine struct {
	config      *config.Config
	lsm         *lsm.LSM
	seqGen      *sequence.SequenceGenerator
	ttlJanitor  *ttl.Janitor
	inMemoryTTL bool
	//wal
}

func NewEngine() (*Engine, error) {
	dataDir := config.GetSettings().SavePath
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}
	lsmTree, err := lsm.NewLSM(dataDir)
	if err != nil {
		return nil, err
	}
	engine := Engine{lsm: lsmTree, inMemoryTTL: config.GetSettings().TTL.InMemoryTTL}

	engine.recover()

	if engine.inMemoryTTL {
		engine.ttlJanitor = ttl.NewTTLJanitor(engine.Delete)
		heap, index, err := engine.lsm.GetAllTTLFomSST()
		if err != nil {
			return nil, err
		}
		engine.ttlJanitor.Init(heap, index)
		go engine.ttlJanitor.Run()
	}
	if config.GetSettings().Debug {
		print("Engine created\n")
	}
	return &engine, nil
}

// check manifest
// check wal
func (engine *Engine) recover() {
	var maxSeq uint64
	maxSeq = engine.lsm.GetMaxSeqId()
	//wal
	engine.seqGen = sequence.NewSequenceGenerator(maxSeq)
}

func (engine *Engine) Put(key []byte, value []byte) {
	seqId := engine.seqGen.Next()
	//wal
	engine.lsm.Put(key, value, seqId, enums.OpTypePut)
}

func (engine *Engine) PutWithTTL(key []byte, value []byte, ttl int64) {
	seqId := engine.seqGen.Next()
	//wal
	if engine.inMemoryTTL {
		engine.ttlJanitor.AddTTL(shared.TTLEntry{ExpiresAt: time.Now().UnixMilli() + ttl, Key: key})
	}
	engine.lsm.PutWithTTL(key, value, seqId, enums.OpTypePut, ttl)
}

func (engine *Engine) Get(key []byte) ([]byte, bool, error) {
	value, found, err := engine.lsm.Get(key)
	return value, found, err
}

func (engine *Engine) GetTTL(key []byte) (int64, bool, error) {
	if !config.GetSettings().TTL.InMemoryTTL {
		value, found, err := engine.lsm.GetTTL(key)
		return value, found, err
	}
	t, found := engine.ttlJanitor.GetTTL(string(key))
	return t, found, nil
}

func (engine *Engine) Delete(key []byte) {
	if config.GetSettings().Debug {
		fmt.Printf("\nDeleting key %s\n", string(key))
	}
	seqId := engine.seqGen.Next()
	// wal
	engine.lsm.Put(key, nil, seqId, enums.OpTypeDel)
}

func (engine *Engine) RangeDelete(startKey []byte, endKey []byte) {
	seqId := engine.seqGen.Next()
	engine.lsm.Put(startKey, endKey, seqId, enums.OpTypeRangeDel)
}

func (engine *Engine) Close() error {
	if engine.inMemoryTTL {
		engine.ttlJanitor.Stop()
	}
	//wal finish write
	err := engine.lsm.Finish()
	if err != nil {
		return err
	}
	return nil
}

func (engine *Engine) ClearAll() error {
	if err := engine.lsm.ClearAll(); err != nil {
		return fmt.Errorf("clear-all: lsm clear failed: %w", err)
	}

	dataDir := config.GetSettings().SavePath
	manifestPath := filepath.Join(dataDir, "MANIFEST")

	if err := os.Remove(manifestPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clear-all: failed to remove manifest: %w", err)
	}
	if engine.inMemoryTTL {
		engine.ttlJanitor.ClearAll()
	}

	return nil
}

func (engine *Engine) DataRaw(index int) {
	basePath := fmt.Sprintf(
		"/home/rikic/.local/share/lsm-kv-engine/%06d.sst",
		index,
	)
	dataPath := basePath + ".data"
	var filePath string
	var readSize int64
	if _, err := os.Stat(dataPath); err == nil {
		filePath = dataPath

		info, err := os.Stat(dataPath)
		if err != nil {
			fmt.Println("stat error:", err)
			return
		}
		readSize = info.Size()

	} else {
		filePath = basePath

		f, err := os.Open(filePath)
		if err != nil {
			fmt.Println("open error:", err)
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			fmt.Println("stat error:", err)
			return
		}
		footerBuf := make([]byte, sstable.FooterSize)
		_, err = f.ReadAt(footerBuf, info.Size()-sstable.FooterSize)
		if err != nil {
			fmt.Println("footer read error:", err)
			return
		}
		footer := &sstable.Footer{}
		footer.Decode(footerBuf)
		readSize = int64(footer.FilterHandler.Offset)
	}
	f, err := os.Open(filePath)
	if err != nil {
		fmt.Println("open error:", err)
		return
	}
	defer f.Close()
	buf := make([]byte, readSize)
	n, err := f.ReadAt(buf, 0)
	if err != nil && err != io.EOF {
		fmt.Println("read error:", err)
		return
	}
	buf = buf[:n]
	fmt.Println("File:", filePath)
	fmt.Println("Data bytes:", buf)
	fmt.Println("-----")
}

// ScanResult is a single key-value pair returned from scan operations
type ScanResult struct {
	Key   string
	Value string
}

// RangeScan returns a page of records whose keys fall within [lower, upper]
// pageNumber is 0-based, pageSize is the number of records per page
func (engine *Engine) RangeScan(lower, upper string, pageNumber, pageSize int) ([]ScanResult, error) {
	if pageSize <= 0 {
		return nil, fmt.Errorf("pageSize must be > 0")
	}
	it, err := engine.lsm.NewRangeIterator([]byte(lower), []byte(upper))
	if err != nil {
		return nil, err
	}
	skip := pageNumber * pageSize
	for i := 0; i < skip && it.Valid(); i++ {
		it.Next()
	}
	results := make([]ScanResult, 0, pageSize)
	for it.Valid() && len(results) < pageSize {
		entry := it.Current()
		results = append(results, ScanResult{
			Key:   string(entry.Key),
			Value: string(entry.Value),
		})
		it.Next()
	}
	return results, nil
}

// PrefixScan returns a page of records whose keys start with the given prefix
// pageNumber is 0-based, pageSize is the number of records per page
func (engine *Engine) PrefixScan(prefix string, pageNumber, pageSize int) ([]ScanResult, error) {
	if pageSize <= 0 {
		return nil, fmt.Errorf("pageSize must be > 0")
	}
	it, err := engine.lsm.NewPrefixIterator([]byte(prefix))
	if err != nil {
		return nil, err
	}
	skip := pageNumber * pageSize
	for i := 0; i < skip && it.Valid(); i++ {
		it.Next()
	}
	results := make([]ScanResult, 0, pageSize)
	for it.Valid() && len(results) < pageSize {
		entry := it.Current()
		results = append(results, ScanResult{
			Key:   string(entry.Key),
			Value: string(entry.Value),
		})
		it.Next()
	}
	return results, nil
}

// ActiveIterator holds an open interactive iterator session
type ActiveIterator struct {
	it iterator.Iterator[iterator.Entry]
}

// Next returns current entry and advances. Returns false if exhausted
func (a *ActiveIterator) Next() (ScanResult, bool) {
	if a.it == nil || !a.it.Valid() {
		return ScanResult{}, false
	}
	entry := a.it.Key()
	result := ScanResult{
		Key:   string(entry.Key),
		Value: string(entry.Value),
	}
	a.it.Next()
	return result, true
}

// Valid reports whether the iterator has more entries
func (a *ActiveIterator) Valid() bool {
	return a.it != nil && a.it.Valid()
}

// RangeIterate creates an interactive iterator over [lower, upper]
func (engine *Engine) RangeIterate(lower, upper string) (*ActiveIterator, error) {
	it, err := engine.lsm.NewRangeIterator([]byte(lower), []byte(upper))
	if err != nil {
		return nil, err
	}
	return &ActiveIterator{it: it}, nil
}

// PrefixIterate creates an interactive iterator over keys starting with prefix
func (engine *Engine) PrefixIterate(prefix string) (*ActiveIterator, error) {
	it, err := engine.lsm.NewPrefixIterator([]byte(prefix))
	if err != nil {
		return nil, err
	}
	return &ActiveIterator{it: it}, nil
}
