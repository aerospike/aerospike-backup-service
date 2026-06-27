package backupexecutor

import (
	"context"
	"log/slog"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/io/storage/options"
	"github.com/aerospike/backup-go/models"
	"golang.org/x/sync/semaphore"
)

type storageWriter interface {
	// CreateDirWriter creates a writer for a folder in the specified storage.
	CreateDirWriter(ctx context.Context, storage model.Storage, path string, opts ...options.Opt) (backup.Writer, error)
}

type credentialsResolver interface {
	Resolve(ctx context.Context, agent *model.SecretAgent, value string) (string, error)
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
		scanLimiter *semaphore.Weighted,
		logger *slog.Logger,
	) (BackupHandler, error)
}

// BackupExecutor selects scan or server backup execution based on the routine policy.
type BackupExecutor struct {
	scan   Backup
	server Backup
}

// NewBackupExecutor creates an executor that delegates to scan or server backup based on backup mode.
func NewBackupExecutor(scan, server Backup) *BackupExecutor {
	return &BackupExecutor{
		scan:   scan,
		server: server,
	}
}

// Run delegates to [ScanBackupExecutor] or [ServerBackupExecutor] based on backup mode.
func (e *BackupExecutor) Run(
	ctx context.Context,
	routine *model.BackupRoutine,
	timeBounds model.TimeBounds,
	namespace string,
	path string,
	scanLimiter *semaphore.Weighted,
	logger *slog.Logger,
) (BackupHandler, error) {
	switch routine.BackupPolicy.GetBackupModeOrDefault() {
	case model.BackupModeServer:
		return e.server.Run(ctx, routine, timeBounds, namespace, path, scanLimiter, logger)
	default:
		return e.scan.Run(ctx, routine, timeBounds, namespace, path, scanLimiter, logger)
	}
}

// closeOnWaitBackupHandler wraps a [BackupHandler] and closes the
// Aerospike client after [BackupHandler.Wait] completes.
type closeOnWaitBackupHandler struct {
	inner         BackupHandler
	client        aerospike.Client
	clientManager aerospike.ClientManager
	closeOnce     sync.Once
}

func newCloseOnWaitBackupHandler(
	handler BackupHandler,
	client aerospike.Client,
	clientManager aerospike.ClientManager,
) *closeOnWaitBackupHandler {
	return &closeOnWaitBackupHandler{
		inner:         handler,
		client:        client,
		clientManager: clientManager,
	}
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
