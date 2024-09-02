package service

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
)

// BackupListReader allows to read list of existing backups.
type BackupListReader interface {
	// FullBackupList returns a list of available full backups.
	// The parameters are timestamp filters by creation time (epoch millis),
	// where from is inclusive and to is exclusive.
	FullBackupList(timebounds *model.TimeBounds) ([]model.BackupDetails, error)

	// IncrementalBackupList returns a list of available incremental backups.
	// The parameters are timestamp filters by creation time (epoch millis),
	// where from is inclusive and to is exclusive.
	IncrementalBackupList(timebounds *model.TimeBounds) ([]model.BackupDetails, error)

	// ReadClusterConfiguration return backed up cluster configuration as a compressed zip.
	ReadClusterConfiguration(path string) ([]byte, error)

	// FindLastFullBackup returns last full backup prior to given time.
	// Each element of an array is backup of a namespace.
	FindLastFullBackup(toTime time.Time) ([]model.BackupDetails, error)

	// FindIncrementalBackupsForNamespace returns all incremental backups in given range, sorted by time.
	FindIncrementalBackupsForNamespace(bounds *model.TimeBounds, namespace string) ([]model.BackupDetails, error)
}
