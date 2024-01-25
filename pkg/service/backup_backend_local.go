package service

import (
	"sync"
	"sync/atomic"
)

// BackupBackend implements the BackupBackend interface by
// saving state to the local file system.
type BackupBackend struct {
	StorageAccessor
	path                 string
	stateFilePath        string
	removeFiles          bool
	fullBackupInProgress *atomic.Bool // BackupBackend needs to know if full backup is running to filter it out
	stateFileMutex       sync.RWMutex
}

const metadataFile = "metadata.yaml"
