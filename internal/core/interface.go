package core

import (
	"github.com/ajromen/LSM-KV-Engine/internal/backup"
	"github.com/ajromen/LSM-KV-Engine/internal/config"
	"github.com/ajromen/LSM-KV-Engine/internal/enums"
	"github.com/ajromen/LSM-KV-Engine/internal/lsm"
	"github.com/ajromen/LSM-KV-Engine/internal/notifier"
	"github.com/ajromen/LSM-KV-Engine/internal/sequence"
	"github.com/ajromen/LSM-KV-Engine/internal/token_bucket"
	"github.com/ajromen/LSM-KV-Engine/internal/ttl"
	"github.com/ajromen/LSM-KV-Engine/internal/wal"
)

type Engine struct {
	config        *config.Config
	lsm           *lsm.LSM
	seqGen        *sequence.SequenceGenerator
	ttlJanitor    *ttl.Janitor
	inMemoryTTL   bool
	notifier      *notifier.Notifier
	tokenBucket   *token_bucket.TokenBucket
	backupManager *backup.BackupManager
	wal           *wal.WAL
}

type EngineInterface interface {
	Put(key []byte, value []byte, seqId uint64, opType enums.OpType) error
	PutWithTTL(key []byte, value []byte, seqId uint64, opType enums.OpType, ttl int64) error
	Get(key []byte) ([]byte, bool, error)
	GetTTL(key []byte) (int64, bool, error)

	RangeScan(lower, upper string, pageNumber, pageSize int) ([]ScanResult, error)
	PrefixScan(prefix string, pageNumber, pageSize int) ([]ScanResult, error)
	RangeIterate(lower, upper string) (*ActiveIterator, error)
	PrefixIterate(prefix string) (*ActiveIterator, error)
	NewPagedRangeScan(lower, upper string, pageSize int) (*PagedRangeScan, error)
	NewPagedPrefixScan(prefix string, pageSize int) (*PagedPrefixScan, error)

	Subscribe(lower, upper string, bufferSize int) *notifier.Listener
	Unsubscribe(l *notifier.Listener)

	GetAllBackups() []string
	CreateBackup(backupType enums.BackupType) (string, error)
	RestoreFromBackup(backupId string) error
	DeleteBackup(backupId string) error
	CascadeDeleteBackup(backupId string) error
	DeleteAllBackups() error

	GetVersions(key []byte) ([]string, error)
	GetVersion(key []byte, version int) (string, bool, error)

	ClearAll() error
	Close() error

	DataRaw(index int)
	WalPrintAll() error
}
