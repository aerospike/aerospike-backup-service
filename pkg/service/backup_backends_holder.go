package service

import (
	"sync"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
)

// BackendsHolder is an interface for storing backup backends.
// We need it because same backends are used in API handlers and backup jobs.
type BackendsHolder interface {
	// GetReader returns BackupBackend for routine as BackupListReader.
	GetReader(routineName string) (BackupListReader, bool)
	// Get returns BackupBackend for routine.
	Get(routineName string) (*BackupBackend, bool)
	// SetData replaces stored backends.
	SetData(backends map[string]*BackupBackend)

	GetAllReaders() map[string]BackupListReader
}

type BackendHolderImpl struct {
	sync.RWMutex
	data map[string]*BackupBackend
}

func (b *BackendHolderImpl) SetData(backends map[string]*BackupBackend) {
	b.Lock()
	defer b.Unlock()
	b.data = backends
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

func NewBackupBackends(config *model.Config) *BackendHolderImpl {
	return &BackendHolderImpl{
		data: BuildBackupBackends(config),
	}
}

func BuildBackupBackends(config *model.Config) map[string]*BackupBackend {
	backends := make(map[string]*BackupBackend, len(config.BackupRoutines))
	for routineName, routine := range config.BackupRoutines {
		backends[routineName] = newBackend(routineName, routine)
	}
	return backends
}
