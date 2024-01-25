package service

import (
	"sync"
	"sync/atomic"
)

// BackupBackendImpl implements the BackupBackend interface by
// saving state to the local file system.
type BackupBackendImpl struct {
	StorageAccessor
	path                 string
	stateFilePath        string
	removeFiles          bool
	fullBackupInProgress *atomic.Bool // BackupBackend needs to know if full backup is running to filter it out
	stateFileMutex       sync.RWMutex
}

var _ BackupBackend = (*BackupBackendImpl)(nil)

const metadataFile = "metadata.yaml"
