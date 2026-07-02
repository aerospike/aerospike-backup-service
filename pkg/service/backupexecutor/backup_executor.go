package backupexecutor

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/syncutil"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/io/storage/options"
	"github.com/aerospike/backup-go/models"
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
		routine *model.BackupRoutine,
		timeBounds model.TimeBounds,
		namespace string,
		path string,
		scanLimiter syncutil.Limiter,
		logger *slog.Logger,
	) (BackupHandler, error)
}

// DefaultBackupExecutor implements [Backup] using backup-go (scan, XDR, or combined) against Aerospike
// and writes backup data via the storage operations facade.
type DefaultBackupExecutor struct {
	clientManager aerospike.ClientManager
	operations    storageWriter
}

// NewDefaultBackupExecutor creates an executor that acquires clients through clientManager and writes via operations.
func NewDefaultBackupExecutor(clientManager aerospike.ClientManager, operations storageWriter) *DefaultBackupExecutor {
	return &DefaultBackupExecutor{
		clientManager: clientManager,
		operations:    operations,
	}
}

// Run implements the backup logic.
// - For regular backups without XDR config, it uses scan-based backup (both full and incremental)
// - For full backups with XDR config, it combines XDR for records and scan for UDFs/indexes
// - For incremental backups with XDR config, it uses XDR-only backup.
// scanLimiter is an optional per-routine scan limiter that limits parallel scans
// within a single routine run, providing fair resource allocation across routines.
func (r *DefaultBackupExecutor) Run(
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
		// Regular scan backup
		handler, err = runScanBackup(ctx, client, routine, timeBounds, namespace, writer)
	case isFullBackup(timeBounds):
		// Full backup with XDR - combine XDR for records and scan for UDFs/indexes
		handler, err = runCombinedBackup(ctx, client, routine, timeBounds, namespace, writer)
	default:
		// Incremental backup with XDR
		handler, err = runXDRBackup(ctx, client, routine, timeBounds, namespace, writer)
	}
	if err != nil {
		r.clientManager.Close(client)
		return nil, err
	}

	return &closeOnWaitBackupHandler{
		inner:         handler,
		client:        client,
		clientManager: r.clientManager,
	}, nil
}

// closeOnWaitBackupHandler wraps a [BackupHandler] and closes the
// Aerospike client after [BackupHandler.Wait] completes.
type closeOnWaitBackupHandler struct {
	inner         BackupHandler
	client        aerospike.Client
	clientManager aerospike.ClientManager
	closeOnce     sync.Once
}

// Wait delegates to the inner handler, then closes the client exactly once.
func (h *closeOnWaitBackupHandler) Wait(ctx context.Context) error {
	defer h.closeOnce.Do(func() { h.clientManager.Close(h.client) })
	return h.inner.Wait(ctx)
}

// GetMetrics returns metrics from the inner backup handler.
func (h *closeOnWaitBackupHandler) GetMetrics() *models.Metrics {
	return h.inner.GetMetrics()
}

// GetStats returns stats from the inner backup handler.
func (h *closeOnWaitBackupHandler) GetStats() *models.BackupStats {
	return h.inner.GetStats()
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
