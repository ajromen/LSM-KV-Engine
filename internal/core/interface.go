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
	//wal
}

type EngineInterface interface {
	Put(key []byte, value []byte)
	PutWithTTL(key []byte, value []byte, ttl int64)
	Get(key []byte) ([]byte, bool, error)
	GetTTL(key []byte) (int64, bool, error)
	Delete(key []byte)
	RangeDelete(startKey []byte, endKey []byte)
	Close() error
	ClearAll() error

	Subscribe(lower, upper string, bufferSize int) *notifier.Listener
	Unsubscribe(l *notifier.Listener)

	RangeScan(lower, upper string, pageNumber, pageSize int) ([]ScanResult, error)
	PrefixScan(prefix string, pageNumber, pageSize int) ([]ScanResult, error)
	RangeIterate(lower, upper string) (*ActiveIterator, error)
	PrefixIterate(prefix string) (*ActiveIterator, error)

	GetAllBackups() []string
	RestoreFromBackup(backupId string) error
	CreateBackup(backupType enums.BackupType) error
	DeleteBackup(backupId string) error
	DeleteAllBackups() error

	DataRaw(index int)
}
