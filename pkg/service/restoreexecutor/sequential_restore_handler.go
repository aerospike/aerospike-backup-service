package restoreexecutor

import (
	"context"
	"sync"

	"github.com/aerospike/backup-go/models"
)

// sequentialRestoreHandler follows restores that run one after another: it reports the stats
// of all of them together, and its Wait ends when the last one ends or one fails.
type sequentialRestoreHandler struct {
	mu       sync.Mutex
	handlers []RestoreHandler
	done     chan struct{}
	err      error
}

var _ RestoreHandler = (*sequentialRestoreHandler)(nil)

func newSequentialRestoreHandler() *sequentialRestoreHandler {
	return &sequentialRestoreHandler{done: make(chan struct{})}
}

// add records a restore that has started.
func (h *sequentialRestoreHandler) add(handler RestoreHandler) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.handlers = append(h.handlers, handler)
}

// finish records the outcome of the whole sequence.
func (h *sequentialRestoreHandler) finish(err error) {
	h.err = err
	close(h.done)
}

// Wait waits for every restore of the sequence to finish, or for the first one to fail.
func (h *sequentialRestoreHandler) Wait(ctx context.Context) error {
	select {
	case <-h.done:
		return h.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// GetStats returns the stats of every restore started so far, added up.
func (h *sequentialRestoreHandler) GetStats() *models.RestoreStats {
	h.mu.Lock()
	defer h.mu.Unlock()

	stats := make([]*models.RestoreStats, 0, len(h.handlers))
	for _, handler := range h.handlers {
		stats = append(stats, handler.GetStats())
	}

	return models.SumRestoreStats(stats...)
}

// GetMetrics returns the metrics of the restore running now, or empty metrics before the first
// one has started.
func (h *sequentialRestoreHandler) GetMetrics() *models.Metrics {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.handlers) == 0 {
		return &models.Metrics{}
	}

	return h.handlers[len(h.handlers)-1].GetMetrics()
}
