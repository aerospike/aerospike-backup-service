package shared

import "github.com/aerospike/backup/pkg/model"

// BackupOptions provides additional properties for running a backup.
type BackupOptions struct {
	ModAfter *int64
}

type Backup interface {
	BackupRun(backupPolicy *model.BackupPolicy, cluster *model.AerospikeCluster,
		storage *model.BackupStorage, opts BackupOptions)
}

type Restore interface {
	RestoreRun(restoreRequest *model.RestoreRequest) bool
}
