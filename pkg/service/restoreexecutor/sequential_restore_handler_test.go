package restoreexecutor

import (
	"context"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// TestSequentialRestoreHandler_Status checks that the handler adds up the counters of the
// restores started so far and reports the metrics of the latest one.
func TestSequentialRestoreHandler_Status(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := newSequentialRestoreHandler(func() {})

	assert.Equal(t, model.RestoreRun{}, h.Status(), "empty before the first restore")

	first := service.NewMockRestoreHandler(ctrl)
	first.EXPECT().Status().Return(model.RestoreRun{
		Counters: model.RestoreCounters{InsertedRecords: 1},
		Metrics:  models.NewMetrics(9, 9, 9, 9),
	})
	latestMetrics := models.NewMetrics(1, 2, 3, 4)
	latest := service.NewMockRestoreHandler(ctrl)
	latest.EXPECT().Status().Return(model.RestoreRun{
		Counters: model.RestoreCounters{InsertedRecords: 2},
		Metrics:  latestMetrics,
	})
	h.add(first)
	h.add(latest)

	run := h.Status()
	assert.Equal(t, uint64(3), run.Counters.InsertedRecords)
	assert.Same(t, latestMetrics, run.Metrics, "the latest restore's metrics")
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
