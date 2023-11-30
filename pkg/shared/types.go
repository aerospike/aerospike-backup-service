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

func (stats BackupStat) IsEmpty() bool {
	return stats.RecordCount == 0 &&
		stats.UDFFileCount == 0 &&
		stats.SecondaryIndexCount == 0
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
