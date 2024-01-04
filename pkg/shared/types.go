package shared

import (
	"github.com/aerospike/backup/pkg/model"
	"time"
)

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

// IsEmpty indicates whether the backup operation represented by the
// BackupStat completed with an empty data set.
func (stats *BackupStat) IsEmpty() bool {
	return stats.RecordCount == 0 &&
		stats.UDFFileCount == 0 &&
		stats.SecondaryIndexCount == 0
}

// Backup represents a backup service.
type Backup interface {
	BackupRun(
		backupRoutine *model.BackupRoutine,
		backupPolicy *model.BackupPolicy,
		cluster *model.AerospikeCluster,
		storage *model.Storage,
		secretAgent *model.SecretAgent,
		opts BackupOptions,
		now time.Time,
	) *BackupStat
}

// Restore represents a restore service.
type Restore interface {
	RestoreRun(restoreRequest *model.RestoreRequestInternal) *model.RestoreResult
}
