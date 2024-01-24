package service

import (
	"github.com/aerospike/backup/pkg/model"
	"sync/atomic"
)

// NewBackupBackendS3 returns a new BackupBackendS3 instance.
func NewBackupBackendS3(storage *model.Storage, backupPolicy *model.BackupPolicy) BackupBackend {
	s3Context, err := NewS3Context(storage)
	if err != nil {
		panic(err)
	}
	return &BackupBackendImpl{
		StorageAccessor:      s3Context,
		path:                 s3Context.path,
		stateFilePath:        s3Context.path + "/" + model.StateFileName,
		backupPolicy:         backupPolicy,
		fullBackupInProgress: &atomic.Bool{},
	}
}
