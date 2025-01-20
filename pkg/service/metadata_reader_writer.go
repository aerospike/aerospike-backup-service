package service

import (
	"context"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// BackupMetadataReader allows to read list of existing backups.
type BackupMetadataReader interface {
	// FullBackupList returns a list of available full backups.
	// The parameters are timestamp filters by creation time (epoch millis),
	// where from is inclusive and to is exclusive.
	FullBackupList(ctx context.Context, timebounds model.TimeBounds) ([]model.BackupDetails, error)

	// IncrementalBackupList returns a list of available incremental backups.
	// The parameters are timestamp filters by creation time (epoch millis),
	// where from is inclusive and to is exclusive.
	IncrementalBackupList(ctx context.Context, timebounds model.TimeBounds) ([]model.BackupDetails, error)

	// ReadClusterConfiguration return backed up cluster configuration as a compressed zip.
	ReadClusterConfiguration(path string) ([]byte, error)

	// FindLastFullBackup returns last full backup prior to given time.
	// Each element of an array is backup of a namespace.
	FindLastFullBackup(toTime time.Time) ([]model.BackupDetails, error)

	// FindIncrementalBackupsForNamespace returns all incremental backups in given range, sorted by time.
	FindIncrementalBackupsForNamespace(
		ctx context.Context, bounds model.TimeBounds, namespace string,
	) ([]model.BackupDetails, error)

	// FindLastRun return timestamps of last full and incremental backups.
	FindLastRun(ctx context.Context) *model.LastBackupRun
}

// BackupMetadataWriter handles backup metadata.
type BackupMetadataWriter interface {
	// writeBackupMetadata writes backup metadata to storage after successful backup.
	writeBackupMetadata(ctx context.Context, path string, metadata model.BackupMetadata) error
	// DeleteFolder deletes a folder with backup data.
	deleteFolder(ctx context.Context, path string) error
}

type BackupMetadataReaderWriter interface {
	BackupMetadataReader
	BackupMetadataWriter
}
