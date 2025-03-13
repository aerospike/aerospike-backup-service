package service

import (
	"context"
	"errors"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// BackupMetadataReader allows to read list of existing backups.
type BackupMetadataReader interface {
	// FullBackupList returns a list of available full backups.
	// The parameters are timestamp filters by creation time (epoch millis),
	// where from is inclusive and to is exclusive.
	FullBackupList(ctx context.Context, timeBounds model.TimeBounds) ([]model.BackupDetails, error)

	// IncrementalBackupList returns a list of available incremental backups.
	// The parameters are timestamp filters by creation time (epoch millis),
	// where from is inclusive and to is exclusive.
	IncrementalBackupList(ctx context.Context, timeBounds model.TimeBounds) ([]model.BackupDetails, error)

	// ReadClusterConfiguration return backed up cluster configuration as a compressed zip.
	ReadClusterConfiguration(ctx context.Context, path string) ([]byte, error)

	// LastFullBackupTime retrieves the time of the most recent full backup in the specified time range.
	LastFullBackupTime(ctx context.Context, timeBounds model.TimeBounds) (time.Time, error)

	// LastIncrementalBackupTime retrieves the time of the most recent incremental backup in the specified time range.
	LastIncrementalBackupTime(ctx context.Context, timeBounds model.TimeBounds) (time.Time, error)

	// FindIncrementalBackupsForNamespace returns all incremental backups in given range, sorted by time.
	FindIncrementalBackupsForNamespace(
		ctx context.Context, bounds model.TimeBounds, namespace string,
	) ([]model.BackupDetails, error)
}

// BackupMetadataWriter provides methods for writing backup metadata and deleting backups.
type BackupMetadataWriter interface {
	// writeBackupMetadata writes backup metadata to storage after successful backup.
	writeBackupMetadata(ctx context.Context, path string, metadata model.BackupMetadata) error
	// DeleteFolder deletes a folder with backup data.
	deleteFolder(ctx context.Context, path string) error
}

// BackupMetadataReaderWriter combines both reading and writing capabilities for backup metadata management.
type BackupMetadataReaderWriter interface {
	BackupMetadataReader
	BackupMetadataWriter
}

var errBackupNotFound = errors.New("backup not found")
