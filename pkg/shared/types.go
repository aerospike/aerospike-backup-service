package shared

import "github.com/aerospike/backup/pkg/model"

// BackupOptions provides additional properties for running a backup.
type BackupOptions struct {
	ModAfter *int64
}

// BackupStat represents partial backup result statistics.
type BackupStat struct {
	RecordCount         int
	SecondaryIndexCount int
	UDFFileCount        int
	HasStats            bool
	Path                string
}

type Backup interface {
	BackupRun(
		backupRoutine *model.BackupRoutine,
		backupPolicy *model.BackupPolicy,
		cluster *model.AerospikeCluster,
		storage *model.Storage,
		opts BackupOptions,
	) *BackupStat
}

type Restore interface {
	RestoreRun(restoreRequest *model.RestoreRequest) bool
}
