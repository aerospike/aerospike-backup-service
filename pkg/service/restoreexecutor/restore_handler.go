package restoreexecutor

import (
	"context"

	"github.com/aerospike/backup-go/models"
)

// RestoreHandler observes a started restore: wait for it to finish and read its statistics.
type RestoreHandler interface {
	// GetStats returns the statistics of the restore job.
	GetStats() *models.RestoreStats
	// Wait waits for the restore job to complete and returns an error if the job failed.
	Wait(context.Context) error
	// GetMetrics returns the performance metrics of the restore job.
	GetMetrics() *models.Metrics
}
