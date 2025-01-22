package service

import (
	"sync"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// BackendsHolder is an interface for storing backup backends.
// We need it because same backends are used in API handlers and backup jobs.
type BackendsHolder interface {
	// Init creates new backends from config.
	Init(routines map[string]*model.BackupRoutine)
	// GetReader returns BackupBackend for routine as BackupMetadataReader.
	GetReader(routineName string) (BackupMetadataReader, bool)
	// Get returns BackupBackend for routine.
	Get(routineName string) (BackupMetadataReaderWriter, bool)
	// GetAllReaders returns all backends as a map routineName -> BackupMetadataReader.
	GetAllReaders() map[string]BackupMetadataReader
}

type BackendHolderImpl struct {
	sync.RWMutex
	data map[string]*BackupBackend
}

func (b *BackendHolderImpl) Init(routines map[string]*model.BackupRoutine) {
	b.Lock()
	defer b.Unlock()

	b.data = make(map[string]*BackupBackend, len(routines))
	for routineName, routine := range routines {
		b.data[routineName] = newBackend(routineName, routine.Storage)
	}
}

var _ BackendsHolder = (*BackendHolderImpl)(nil)

func (b *BackendHolderImpl) GetReader(name string) (BackupMetadataReader, bool) {
	b.RLock()
	defer b.RUnlock()
	backend, found := b.data[name]
	return backend, found
}

func (b *BackendHolderImpl) GetAllReaders() map[string]BackupMetadataReader {
	b.RLock()
	defer b.RUnlock()

	readers := make(map[string]BackupMetadataReader, len(b.data))
	for name, backend := range b.data {
		readers[name] = backend
	}

	return readers
}

func (b *BackendHolderImpl) Get(name string) (BackupMetadataReaderWriter, bool) {
	b.RLock()
	defer b.RUnlock()
	backend, found := b.data[name]
	return backend, found
}

func NewBackupBackends() *BackendHolderImpl {
	return &BackendHolderImpl{}
}
