package service

import (
	"sync"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
)

// BackendsHolder is an interface for storing backup backends.
// We need it because same backends are used in API handlers and backup jobs.
type BackendsHolder interface {
	// Init creates new backends from config.
	Init(config *model.Config)
	// GetReader returns BackupBackend for routine as BackupListReader.
	GetReader(routineName string) (BackupListReader, bool)
	// Get returns BackupBackend for routine.
	Get(routineName string) (*BackupBackend, bool)
	// GetAllReaders returns all backends as a map routineName -> BackupListReader.
	GetAllReaders() map[string]BackupListReader
}

type BackendHolderImpl struct {
	sync.RWMutex
	data map[string]*BackupBackend
}

func (b *BackendHolderImpl) Init(config *model.Config) {
	b.Lock()
	defer b.Unlock()

	routines := config.BackupRoutines
	b.data = make(map[string]*BackupBackend, len(routines))
	for routineName, routine := range routines {
		b.data[routineName] = newBackend(routineName, routine)
	}
}

var _ BackendsHolder = (*BackendHolderImpl)(nil)

func (b *BackendHolderImpl) GetReader(name string) (BackupListReader, bool) {
	b.RLock()
	defer b.RUnlock()
	backend, found := b.data[name]
	return backend, found
}

func (b *BackendHolderImpl) GetAllReaders() map[string]BackupListReader {
	b.RLock()
	defer b.RUnlock()

	readers := make(map[string]BackupListReader, len(b.data))
	for name, backend := range b.data {
		readers[name] = backend
	}

	return readers
}

func (b *BackendHolderImpl) Get(name string) (*BackupBackend, bool) {
	b.RLock()
	defer b.RUnlock()
	backend, found := b.data[name]
	return backend, found
}

func NewBackupBackends() *BackendHolderImpl {
	return &BackendHolderImpl{}
}
