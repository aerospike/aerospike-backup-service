package service

import (
	"errors"
	"testing"
	"time"

	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestBackupNamespacesOperation_Wait_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	h1 := NewMockCancelableBackupHandler(ctrl)
	h1.EXPECT().Wait(gomock.Any()).Return(nil)
	h2 := NewMockCancelableBackupHandler(ctrl)
	h2.EXPECT().Wait(gomock.Any()).Return(nil)

	op := &BackupNamespacesOperation{handlers: map[string]CancelableBackupHandler{"ns1": h1, "ns2": h2}}
	require.NoError(t, op.Wait(t.Context()))
}

func TestBackupNamespacesOperation_Wait_AggregatesErrors(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	h1 := NewMockCancelableBackupHandler(ctrl)
	h1.EXPECT().Wait(gomock.Any()).Return(errors.New("ns1 failed"))
	h2 := NewMockCancelableBackupHandler(ctrl)
	h2.EXPECT().Wait(gomock.Any()).Return(errors.New("ns2 failed"))

	op := &BackupNamespacesOperation{handlers: map[string]CancelableBackupHandler{"ns1": h1, "ns2": h2}}
	err := op.Wait(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ns1 failed")
	assert.Contains(t, err.Error(), "ns2 failed")
}

func TestBackupNamespacesOperation_Cancel(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	h1 := NewMockCancelableBackupHandler(ctrl)
	h1.EXPECT().Cancel()
	h2 := NewMockCancelableBackupHandler(ctrl)
	h2.EXPECT().Cancel()

	op := &BackupNamespacesOperation{handlers: map[string]CancelableBackupHandler{"ns1": h1, "ns2": h2}}
	op.Cancel()
}

func TestBackupNamespacesOperation_Status_NoneStarted(t *testing.T) {
	t.Parallel()

	h := NewMockCancelableBackupHandler(gomock.NewController(t))
	h.EXPECT().GetStats().Return(nil)

	op := &BackupNamespacesOperation{handlers: map[string]CancelableBackupHandler{"ns1": h}}

	assert.Nil(t, op.Status())
}

func TestBackupNamespacesOperation_Status_Aggregated(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	stats1 := models.NewBackupStats()
	stats1.TotalRecords.Store(10)
	stats1.ReadRecords.Add(5)
	stats1.StartTime = start
	h1 := NewMockCancelableBackupHandler(ctrl)
	h1.EXPECT().GetStats().Return(stats1)
	h1.EXPECT().GetMetrics().Return(&models.Metrics{RecordsPerSecond: 10})

	stats2 := models.NewBackupStats()
	stats2.TotalRecords.Store(20)
	stats2.ReadRecords.Add(15)
	// Same StartTime as stats1: all namespaces in a routine start together, so the
	// aggregation result is deterministic regardless of map iteration order.
	stats2.StartTime = start
	h2 := NewMockCancelableBackupHandler(ctrl)
	h2.EXPECT().GetStats().Return(stats2)
	h2.EXPECT().GetMetrics().Return(&models.Metrics{RecordsPerSecond: 20})

	// Not started yet: excluded from the counts.
	h3 := NewMockCancelableBackupHandler(ctrl)
	h3.EXPECT().GetStats().Return(nil)

	op := &BackupNamespacesOperation{
		handlers: map[string]CancelableBackupHandler{"ns1": h1, "ns2": h2, "ns3": h3},
	}

	run := op.Status()
	require.NotNil(t, run)
	assert.Equal(t, start, run.StartTime)
	assert.InDelta(t, 20.0/30.0, run.Progress, 1e-9)
	assert.Equal(t, uint64(30), run.Scan.TotalRecords)
	assert.Equal(t, uint64(20), run.Scan.DoneRecords)
	assert.Equal(t, uint64(30), run.Scan.Metrics.RecordsPerSecond)
}

// A started namespace with no metrics yet still yields zero-valued metrics, not nil: the REST
// field is {} rather than null.
func TestBackupNamespacesOperation_Status_NoMetricsYet(t *testing.T) {
	t.Parallel()

	h := NewMockCancelableBackupHandler(gomock.NewController(t))
	h.EXPECT().GetStats().Return(models.NewBackupStats())
	h.EXPECT().GetMetrics().Return(nil)
	op := &BackupNamespacesOperation{handlers: map[string]CancelableBackupHandler{"ns1": h}}

	run := op.Status()

	require.NotNil(t, run)
	require.NotNil(t, run.Scan.Metrics)
	assert.Zero(t, run.Scan.Metrics.RecordsPerSecond)
}

func TestBackupNamespacesOperation_Status_Progress(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		total, read  uint64
		wantProgress float64
	}{
		"part read":         {total: 100, read: 25, wantProgress: 0.25},
		"not estimated yet": {total: 0, read: 0, wantProgress: 0},
		// Only the progress is capped; the counts stay as read.
		"estimate fell short, capped": {total: 100, read: 107, wantProgress: 1},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			stats := models.NewBackupStats()
			stats.TotalRecords.Store(tt.total)
			stats.ReadRecords.Store(tt.read)
			h := NewMockCancelableBackupHandler(gomock.NewController(t))
			h.EXPECT().GetStats().Return(stats)
			h.EXPECT().GetMetrics().Return(&models.Metrics{})

			op := &BackupNamespacesOperation{handlers: map[string]CancelableBackupHandler{"ns1": h}}

			run := op.Status()

			require.NotNil(t, run)
			assert.InDelta(t, tt.wantProgress, run.Progress, 1e-9)
			assert.Equal(t, tt.total, run.Scan.TotalRecords)
			assert.Equal(t, tt.read, run.Scan.DoneRecords)
		})
	}
}
