package serverrestore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/restoreexecutor"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/models"
	infoModels "github.com/aerospike/backup-go/pkg/asinfo/models"
)

const pollInterval = time.Second

// Handler monitors a server-side restore job started via Aerospike info commands.
type Handler struct {
	infoClient backup.ServerBackupInfo
	namespace  string
	stats      *models.RestoreStats
	waitErr    error
}

var _ restoreexecutor.RestoreHandler = (*Handler)(nil)

func newHandler(infoClient backup.ServerBackupInfo, namespace string) *Handler {
	return &Handler{
		infoClient: infoClient,
		namespace:  namespace,
		stats:      models.NewRestoreStats(),
	}
}

// Wait polls cluster stability until the restore completes or the context is canceled.
func (h *Handler) Wait(ctx context.Context) error {
	if h == nil {
		return nil
	}

	if h.stats.StartTime.IsZero() {
		h.stats.Start()
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		stable, err := h.infoClient.GetRestoreStatus(ctx, h.namespace)
		if err != nil {
			h.waitErr = fmt.Errorf("failed to get cluster stable status: %w", err)
			h.stats.Stop()
			return h.waitErr
		}

		if stable == infoModels.RestoreStateNone { // restore completed
			h.stats.Stop()
			return nil
		}

		if stable == infoModels.RestoreStateFailed {
			h.waitErr = errors.New("restore failed")
			h.stats.Stop()

			return h.waitErr
		}

		select {
		case <-ctx.Done():
			h.waitErr = ctx.Err()
			h.stats.Stop()
			return h.waitErr
		case <-ticker.C:
		}
	}
}

// GetStats returns restore statistics tracked by the service.
func (h *Handler) GetStats() *models.RestoreStats {
	if h == nil {
		return nil
	}

	return h.stats
}

// GetMetrics returns nil because server-side restore does not expose client-side throughput metrics.
func (h *Handler) GetMetrics() *models.Metrics {
	return nil
}
