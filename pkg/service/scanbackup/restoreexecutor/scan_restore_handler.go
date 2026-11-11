package restoreexecutor

import (
	"context"
	"sync/atomic"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/models"
)

// scanRestoreHandler adapts backup-go's restore handler to [service.RestoreHandler].
type scanRestoreHandler struct {
	handler backup.RestoreHandler
	// finished is set once Wait has returned.
	finished atomic.Bool
}

var _ service.RestoreHandler = (*scanRestoreHandler)(nil)

func (h *scanRestoreHandler) Wait(ctx context.Context) error {
	defer h.finished.Store(true)

	return h.handler.Wait(ctx)
}

// Status reports the counters and the throughput backup-go measures. A finished restore reports
// no throughput. backup-go keeps the last rate it measured, and a job adds up the throughput of
// its handlers, so the rates of the finished backups of a chain would be added to the rate of
// the running one.
func (h *scanRestoreHandler) Status() model.RestoreRun {
	run := model.RestoreRun{Counters: countersOf(h.handler.GetStats())}
	if h.finished.Load() {
		return run
	}

	run.Metrics = h.handler.GetMetrics()

	return run
}

func countersOf(stats *models.RestoreStats) model.RestoreCounters {
	return model.RestoreCounters{
		ReadRecords:      stats.GetReadRecords(),
		InsertedRecords:  stats.GetRecordsInserted(),
		SkippedRecords:   stats.GetRecordsSkipped(),
		ExpiredRecords:   stats.GetRecordsExpired(),
		IgnoredRecords:   stats.GetRecordsIgnored(),
		ExistedRecords:   stats.GetRecordsExisted(),
		FresherRecords:   stats.GetRecordsFresher(),
		ErrorsInDoubt:    stats.GetErrorsInDoubt(),
		TotalBytesRead:   stats.GetTotalBytesRead(),
		BytesWritten:     stats.GetBytesWritten(),
		SecondaryIndexes: uint64(stats.GetSIndexes()),
		UDFs:             uint64(stats.GetUDFs()),
	}
}
