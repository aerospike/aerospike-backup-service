package service

import (
	"github.com/aerospike/backup/pkg/model"
	"sync"
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

func (b *BackendHolderImpl) Get(name string) (*BackupBackend, bool) {
	b.RLock()
	defer b.RUnlock()
	backend, found := b.data[name]
	return backend, found
}

func NewBackupBackends(config *model.Config) *BackendHolderImpl {
	return &BackendHolderImpl{
		RWMutex: sync.RWMutex{},
		data:    BuildBackupBackends(config),
	}
}

func BuildBackupBackends(config *model.Config) map[string]*BackupBackend {
	backends := map[string]*BackupBackend{}
	for routineName := range config.BackupRoutines {
		backends[routineName] = newBackend(config, routineName)
	}
	return backends
}
