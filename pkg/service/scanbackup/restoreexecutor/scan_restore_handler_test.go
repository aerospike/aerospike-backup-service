package restoreexecutor

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// TestCountersOf gives every backup-go statistic its own value, so that two counters that swap
// their sources fail the test.
func TestCountersOf(t *testing.T) {
	t.Parallel()

	stats := models.NewRestoreStats()
	stats.ReadRecords.Store(1)
	for range 2 {
		stats.IncrRecordsInserted()
	}
	stats.RecordsSkipped.Store(3)
	stats.RecordsExpired.Store(4)
	stats.RecordsIgnored.Store(5)
	for range 6 {
		stats.IncrRecordsExisted()
	}
	for range 7 {
		stats.IncrRecordsFresher()
	}
	stats.AddErrorsInDoubt(8)
	stats.TotalBytesRead.Store(9)
	stats.BytesWritten.Store(10)
	stats.AddSIndexes(11)
	stats.AddUDFs(12)

	assert.Equal(t, model.RestoreCounters{
		ReadRecords:      1,
		InsertedRecords:  2,
		SkippedRecords:   3,
		ExpiredRecords:   4,
		IgnoredRecords:   5,
		ExistedRecords:   6,
		FresherRecords:   7,
		ErrorsInDoubt:    8,
		TotalBytesRead:   9,
		BytesWritten:     10,
		SecondaryIndexes: 11,
		UDFs:             12,
	}, countersOf(stats))
}

// TestScanRestoreHandler_Status checks that a restore reports its throughput only while it
// runs. backup-go keeps the last rate it measured, and the job adds the rates of its handlers up.
func TestScanRestoreHandler_Status(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	stats := models.NewRestoreStats()
	stats.ReadRecords.Store(3)
	metrics := &models.Metrics{RecordsPerSecond: 100}

	backupGoHandler := NewMockRestoreHandler(ctrl)
	backupGoHandler.EXPECT().GetStats().Return(stats).AnyTimes()
	backupGoHandler.EXPECT().GetMetrics().Return(metrics).AnyTimes()
	backupGoHandler.EXPECT().Wait(gomock.Any()).Return(nil)

	handler := &scanRestoreHandler{handler: backupGoHandler}
	counters := model.RestoreCounters{ReadRecords: 3}

	assert.Equal(t, model.RestoreRun{Counters: counters, Metrics: metrics}, handler.Status())

	require.NoError(t, handler.Wait(t.Context()))

	assert.Equal(t, model.RestoreRun{Counters: counters}, handler.Status())
}
