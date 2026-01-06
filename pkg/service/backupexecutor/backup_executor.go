package backupexecutor

import (
	"context"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/io/storage/options"
)

type storageWriter interface {
	// CreateDirWriter creates a writer for a folder in the specified storage.
	CreateDirWriter(ctx context.Context, storage model.Storage, path string, opts ...options.Opt) (backup.Writer, error)
}

// Backup defines the interface for running backups.
type Backup interface {
	// Run runs the backup and returns a handler for monitoring progress.
	Run(
		ctx context.Context,
		client aerospike.Backuper,
		routine *model.BackupRoutine,
		timeBounds model.TimeBounds,
		namespace string,
		path string,
	) (BackupHandler, error)
}

// DefaultBackupExecutor implements the actual backup logic.
type DefaultBackupExecutor struct {
	operations storageWriter
}

func NewDefaultBackupExecutor(operations storageWriter) *DefaultBackupExecutor {
	return &DefaultBackupExecutor{
		operations: operations,
	}
}

// Run implements the backup logic.
// - For regular backups without XDR config, it uses scan-based backup (both full and incremental)
// - For full backups with XDR config, it combines XDR for records and scan for UDFs/indexes
// - For incremental backups with XDR config, it uses XDR-only backup.
func (r *DefaultBackupExecutor) Run(
	ctx context.Context,
	client aerospike.Backuper,
	routine *model.BackupRoutine,
	timeBounds model.TimeBounds,
	namespace string,
	path string,
) (BackupHandler, error) {
	xdrEnabled := routine.BackupPolicy.XDRConfig != nil
	withStorageClass := options.WithStorageClass(routine.Storage.GetStorageClass().DataClass)
	writer, err := r.operations.CreateDirWriter(ctx, routine.Storage, path, withStorageClass)
	if err != nil {
		return nil, fmt.Errorf("failed to create backup writer: %w", err)
	}

	switch {
	case !xdrEnabled:
		// Regular scan backup
		return runScanBackup(ctx, client, routine, timeBounds, namespace, writer)
	case isFullBackup(timeBounds):
		// Full backup with XDR - combine XDR for records and scan for UDFs/indexes
		return runCombinedBackup(ctx, client, routine, timeBounds, namespace, writer)
	default:
		// Incremental backup with XDR
		return runXDRBackup(ctx, client, routine, timeBounds, namespace, writer)
	}
}

func isFullBackup(timeBounds model.TimeBounds) bool {
	return timeBounds.FromTime == nil
}

// runCombinedBackup performs both XDR backup for records and scan backup for UDFs/indexes.
func runCombinedBackup(
	ctx context.Context,
	client aerospike.Backuper,
	routine *model.BackupRoutine,
	timeBounds model.TimeBounds,
	namespace string,
	writer backup.Writer,
) (BackupHandler, error) {
	xdrHandler, err := runXDRBackup(ctx, client, routine, timeBounds, namespace, writer)
	if err != nil {
		return nil, fmt.Errorf("failed to start XDR backup: %w", err)
	}

	// For scan backup, create a copy of routine with NoRecords set to true.
	scanRoutine := *routine
	scanRoutine.BackupPolicy = routine.BackupPolicy.CopyWithNoRecords()

	scanHandler, err := runScanBackup(ctx, client, &scanRoutine, timeBounds, namespace, writer)
	if err != nil {
		return nil, fmt.Errorf("failed to start scan backup: %w", err)
	}

	return &CombinedBackupHandler{
		xdrHandler:  xdrHandler,
		scanHandler: scanHandler,
	}, nil
}
