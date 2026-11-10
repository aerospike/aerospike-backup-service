package restoreexecutor

import (
	"context"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/models"
)

// scanRestoreHandler adapts backup-go's restore handler to [service.RestoreHandler].
type scanRestoreHandler struct {
	handler backup.RestoreHandler
}

var _ service.RestoreHandler = scanRestoreHandler{}

func (h scanRestoreHandler) Wait(ctx context.Context) error {
	return h.handler.Wait(ctx)
}

// Status reports the counters and the throughput backup-go measures. It reports no progress,
// because the progress of a scan restore follows from the record counts.
func (h scanRestoreHandler) Status() model.RestoreRun {
	return model.RestoreRun{
		Counters: countersOf(h.handler.GetStats()),
		Metrics:  h.handler.GetMetrics(),
	}
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
