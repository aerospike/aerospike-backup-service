package backupexecutor

import (
	"context"
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

// Backup defines the interface for running scan-based backups.
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

// closeOnWaitBackupHandler wraps a [BackupHandler] and closes the
// Aerospike client after [BackupHandler.Wait] completes.
type closeOnWaitBackupHandler struct {
	inner         BackupHandler
	client        aerospike.Client
	clientManager aerospike.ClientManager
	closeOnce     sync.Once
}

// newCloseOnWaitBackupHandler wraps handler and ensures the Aerospike client is closed after Wait.
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
