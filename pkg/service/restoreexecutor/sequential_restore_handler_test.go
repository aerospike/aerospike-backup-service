package restoreexecutor

import (
	"context"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSequentialRestoreHandler_GetMetrics(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := newSequentialRestoreHandler(func() {})

	assert.Equal(t, &models.Metrics{}, h.GetMetrics(), "empty before the first restore")

	latestMetrics := models.NewMetrics(1, 2, 3, 4)
	latest := service.NewMockRestoreHandler(ctrl)
	latest.EXPECT().GetMetrics().Return(latestMetrics)
	h.add(service.NewMockRestoreHandler(ctrl))
	h.add(latest)

	assert.Same(t, latestMetrics, h.GetMetrics(), "the latest restore's metrics")
}

func TestSequentialRestoreHandler_Wait(t *testing.T) {
	t.Run("returns the outcome once finished", func(t *testing.T) {
		h := newSequentialRestoreHandler(func() {})
		h.finish(nil)

		require.NoError(t, h.Wait(t.Context()))
	})

	t.Run("canceled wait stops the sequence and returns its outcome", func(t *testing.T) {
		// The sequence ends with the cancellation, as a real one does once its context ends.
		var h *sequentialRestoreHandler
		h = newSequentialRestoreHandler(func() { h.finish(context.Canceled) })
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		require.ErrorIs(t, h.Wait(ctx), context.Canceled)
	})
}
