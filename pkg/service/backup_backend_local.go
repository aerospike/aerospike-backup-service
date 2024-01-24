package service

import (
	"sync"
	"sync/atomic"

	"github.com/aerospike/backup/pkg/model"
)

// BackupBackendImpl implements the BackupBackend interface by
// saving state to the local file system.
type BackupBackendImpl struct {
	StorageAccessor
	path                 string
	stateFilePath        string
	backupPolicy         *model.BackupPolicy
	fullBackupInProgress *atomic.Bool // BackupBackend needs to know if full backup is running to filter it out
	stateFileMutex       sync.RWMutex
}

var _ BackupBackend = (*BackupBackendImpl)(nil)

const metadataFile = "metadata.yaml"

// NewBackupBackendLocal returns a new BackupBackendImpl instance.
func NewBackupBackendLocal(storage *model.Storage, backupPolicy *model.BackupPolicy) BackupBackend {
	path := *storage.Path
	diskAccessor := NewOS(path)
	return &BackupBackendImpl{
		StorageAccessor:      diskAccessor,
		path:                 path,
		stateFilePath:        path + "/" + model.StateFileName,
		backupPolicy:         backupPolicy,
		fullBackupInProgress: &atomic.Bool{},
	}
}
