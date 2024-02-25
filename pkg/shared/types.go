package shared

import (
	"time"

	"github.com/aerospike/backup/pkg/model"
)

// BackupOptions provides additional properties for running a backup.
type BackupOptions struct {
	ModBefore *int64
	ModAfter  *int64
}

// BackupStat represents partial backup result statistics returned from asbackup library.
type BackupStat struct {
	RecordCount uint64
	ByteCount   uint64
	FileCount   uint64
	IndexCount  uint64
	UDFCount    uint64
}

// IsEmpty indicates whether the backup operation represented by the
// BackupStat completed with an empty data set.
func (stats *BackupStat) IsEmpty() bool {
	return stats.RecordCount == 0 &&
		stats.UDFCount == 0 &&
		stats.IndexCount == 0
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
		namespace *string,
		path *string,
	) *BackupStat
}

// Restore represents a restore service.
type Restore interface {
	RestoreRun(restoreRequest *model.RestoreRequestInternal) *model.RestoreResult
}

func (stats *BackupStat) ToMetadata(from, created time.Time, namespace string) model.BackupMetadata {
	return model.BackupMetadata{
		Created:             created,
		From:                from,
		Namespace:           namespace,
		RecordCount:         stats.RecordCount,
		FileCount:           stats.FileCount,
		ByteCount:           stats.ByteCount,
		SecondaryIndexCount: stats.IndexCount,
		UDFCount:            stats.UDFCount,
	}
}
