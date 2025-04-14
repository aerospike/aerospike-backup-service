package restoreexecutor

import (
	"context"
	"fmt"

	"github.com/aerospike/backup-go/models"
)

// CombinedRestoreHandler combines two restore handlers into one.
type CombinedRestoreHandler struct {
	streamHandler RestoreHandler
	xdrHandler    RestoreHandler
}

func NewCombinedRestoreHandler(streamHandler RestoreHandler, xdrHandler RestoreHandler) *CombinedRestoreHandler {
	return &CombinedRestoreHandler{
		streamHandler: streamHandler,
		xdrHandler:    xdrHandler,
	}
}

var _ RestoreHandler = (*CombinedRestoreHandler)(nil)

func (h *CombinedRestoreHandler) Wait(ctx context.Context) error {
	if err := h.streamHandler.Wait(ctx); err != nil {
		return fmt.Errorf("streaming restore failed: %w", err)
	}

	if err := h.xdrHandler.Wait(ctx); err != nil {
		return fmt.Errorf("XDR restore failed: %w", err)
	}

	return nil
}

func (h *CombinedRestoreHandler) GetStats() *models.RestoreStats {
	return models.SumRestoreStats(h.streamHandler.GetStats(), h.xdrHandler.GetStats())
}

func (h *CombinedRestoreHandler) GetMetrics() *models.Metrics {
	stream := h.streamHandler.GetMetrics()
	xdr := h.xdrHandler.GetMetrics()
	return models.NewMetrics(
		stream.PipelineReadQueueSize+xdr.PipelineReadQueueSize,
		stream.PipelineWriteQueueSize+xdr.PipelineWriteQueueSize,
	)
}
