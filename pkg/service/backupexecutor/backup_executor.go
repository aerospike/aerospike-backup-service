package backupexecutor

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/syncutil"
	"github.com/aerospike/backup-go/io/storage/options"
	"github.com/aerospike/backup-go/models"
)

// BackupHandler observes a started backup: wait for it to finish and read its statistics.
type BackupHandler interface {
	// GetStats returns backup job statistics.
	//
	// Nil means the job has not started yet. Wrappers (e.g. retryableBackupHandler)
	// return nil until their inner pipeline is running. backup-go handlers never
	// return nil once constructed.
	GetStats() *models.BackupStats
	// Wait waits for the backup job to complete and returns an error if the job failed.
	Wait(context.Context) error
	// GetMetrics returns the performance metrics of the backup job.
	GetMetrics() *models.Metrics
}

// Backup starts the backup of a single namespace and returns a handle to observe it.
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

// BackupExecutor runs scan backups with backup-go and writes the data through storage operations.
type BackupExecutor struct {
	clientManager aerospike.ClientManager
	operations    storage.Operations
}

var _ Backup = (*BackupExecutor)(nil)

// NewBackupExecutor returns a BackupExecutor.
func NewBackupExecutor(clientManager aerospike.ClientManager, operations storage.Operations) *BackupExecutor {
	return &BackupExecutor{
		clientManager: clientManager,
		operations:    operations,
	}
}

// Run starts a scan backup of one namespace.
// scanLimiter is an optional per-routine scan limiter that limits parallel scans
// within a single routine run, providing fair resource allocation across routines.
func (r *BackupExecutor) Run(
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

	withStorageClass := options.WithStorageClass(routine.Storage.GetStorageClass().DataClass)
	writer, err := r.operations.CreateDirWriter(ctx, routine.Storage, path, withStorageClass)
	if err != nil {
		r.clientManager.Close(client)
		return nil, fmt.Errorf("failed to create backup writer: %w", err)
	}

	handler, err := runScanBackup(ctx, client, routine, timeBounds, namespace, writer)
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
