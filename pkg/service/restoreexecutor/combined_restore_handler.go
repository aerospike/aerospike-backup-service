package restoreexecutor

import (
	"context"
	"fmt"

	"github.com/aerospike/backup-go/models"
)

// CombinedRestoreHandler combines multiple restore handlers into one.
type CombinedRestoreHandler struct {
	handlers []RestoreHandler
}

func NewCombinedRestoreHandler(handlers ...RestoreHandler) *CombinedRestoreHandler {
	validHandlers := make([]RestoreHandler, 0, len(handlers))
	for _, h := range handlers {
		if h != nil {
			validHandlers = append(validHandlers, h)
		}
	}

	return &CombinedRestoreHandler{
		handlers: validHandlers,
	}
}

var _ RestoreHandler = (*CombinedRestoreHandler)(nil)

func (h *CombinedRestoreHandler) Wait(ctx context.Context) error {
	for _, handler := range h.handlers {
		if err := handler.Wait(ctx); err != nil {
			return fmt.Errorf("restore failed: %w", err)
		}
	}
	return nil
}

func (h *CombinedRestoreHandler) GetStats() *models.RestoreStats {
	stats := make([]*models.RestoreStats, 0, len(h.handlers))
	for _, handler := range h.handlers {
		stats = append(stats, handler.GetStats())
	}
	return models.SumRestoreStats(stats...)
}

func (h *CombinedRestoreHandler) GetMetrics() *models.Metrics {
	metrics := make([]*models.Metrics, 0, len(h.handlers))
	for _, handler := range h.handlers {
		metrics = append(metrics, handler.GetMetrics())
	}
	return models.SumMetrics(metrics...)
}
