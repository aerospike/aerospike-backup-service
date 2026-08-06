package backupexecutor

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/syncutil"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/io/storage/options"
)

// ScanBackupExecutor implements [Backup] using backup-go scan, XDR, or combined backup
// and writes backup data via the storage operations facade.
type ScanBackupExecutor struct {
	clientManager aerospike.ClientManager
	operations    storageWriter
}

// NewScanBackupExecutor creates an executor that acquires clients through clientManager and writes via operations.
func NewScanBackupExecutor(clientManager aerospike.ClientManager, operations storageWriter) *ScanBackupExecutor {
	return &ScanBackupExecutor{
		clientManager: clientManager,
		operations:    operations,
	}
}

// Run implements the scan-based backup logic.
// - For regular backups without XDR config, it uses scan-based backup (both full and incremental)
// - For full backups with XDR config, it combines XDR for records and scan for UDFs/indexes
// - For incremental backups with XDR config, it uses XDR-only backup.
// scanLimiter is an optional per-routine scan limiter that limits parallel scans
// within a single routine run, providing fair resource allocation across routines.
func (r *ScanBackupExecutor) Run(
	ctx context.Context,
	routine *model.BackupRoutine,
	timeBounds model.TimeBounds,
	namespace string,
	path string,
	scanLimiter syncutil.Limiter,
	logger *slog.Logger,
) (BackupHandler, error) {
	client, err := r.clientManager.GetClient(ctx, routine.SourceCluster, scanLimiter, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to get backup client: %w", err)
	}

	xdrEnabled := routine.BackupPolicy.XDRConfig != nil
	withStorageClass := options.WithStorageClass(routine.Storage.GetStorageClass().DataClass)
	writer, err := r.operations.CreateDirWriter(ctx, routine.Storage, path, withStorageClass)
	if err != nil {
		r.clientManager.Close(client)
		return nil, fmt.Errorf("failed to create backup writer: %w", err)
	}

	var handler BackupHandler
	switch {
	case !xdrEnabled:
		handler, err = runScanBackup(ctx, client, routine, timeBounds, namespace, writer)
	case isFullBackup(timeBounds):
		handler, err = runCombinedBackup(ctx, client, routine, timeBounds, namespace, writer)
	default:
		handler, err = runXDRBackup(ctx, client, routine, timeBounds, namespace, writer)
	}
	if err != nil {
		r.clientManager.Close(client)
		return nil, err
	}

	return newCloseOnWaitBackupHandler(handler, client, r.clientManager), nil
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
