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
	defer ctrl.Finish()

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
	defer ctrl.Finish()

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
	defer ctrl.Finish()

	h1 := NewMockCancelableBackupHandler(ctrl)
	h1.EXPECT().Cancel()
	h2 := NewMockCancelableBackupHandler(ctrl)
	h2.EXPECT().Cancel()

	op := &BackupNamespacesOperation{handlers: map[string]CancelableBackupHandler{"ns1": h1, "ns2": h2}}
	op.Cancel()
}

func TestBackupNamespacesOperation_GetMetrics(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h1 := NewMockCancelableBackupHandler(ctrl)
	h1.EXPECT().GetMetrics().Return(&models.Metrics{RecordsPerSecond: 10})
	h2 := NewMockCancelableBackupHandler(ctrl)
	h2.EXPECT().GetMetrics().Return(&models.Metrics{RecordsPerSecond: 20})

	op := &BackupNamespacesOperation{handlers: map[string]CancelableBackupHandler{"ns1": h1, "ns2": h2}}
	metrics := op.GetMetrics()
	require.NotNil(t, metrics)
	assert.Equal(t, uint64(30), metrics.RecordsPerSecond)
}

func TestBackupNamespacesOperation_GetStats_NoneStarted(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h1 := NewMockCancelableBackupHandler(ctrl)
	h1.EXPECT().GetStats().Return(nil)

	op := &BackupNamespacesOperation{handlers: map[string]CancelableBackupHandler{"ns1": h1}}
	assert.Nil(t, op.GetStats())
}

func TestBackupNamespacesOperation_GetStats_Aggregated(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	stats1 := models.NewBackupStats()
	stats1.TotalRecords.Store(10)
	stats1.ReadRecords.Add(5)
	stats1.BytesWritten.Add(100)
	stats1.StartTime = start

	stats2 := models.NewBackupStats()
	stats2.TotalRecords.Store(20)
	stats2.ReadRecords.Add(15)
	stats2.BytesWritten.Add(200)
	// Same StartTime as stats1: all namespaces in a routine start together, so the
	// aggregation result is deterministic regardless of map iteration order.
	stats2.StartTime = start

	h1 := NewMockCancelableBackupHandler(ctrl)
	h1.EXPECT().GetStats().Return(stats1)
	h2 := NewMockCancelableBackupHandler(ctrl)
	h2.EXPECT().GetStats().Return(stats2)
	// Not started yet: excluded from aggregation.
	h3 := NewMockCancelableBackupHandler(ctrl)
	h3.EXPECT().GetStats().Return(nil)

	op := &BackupNamespacesOperation{
		handlers: map[string]CancelableBackupHandler{"ns1": h1, "ns2": h2, "ns3": h3},
	}
	result := op.GetStats()
	require.NotNil(t, result)
	assert.Equal(t, uint64(30), result.TotalRecords.Load())
	assert.Equal(t, uint64(20), result.GetReadRecords())
	assert.Equal(t, start, result.StartTime)
}
