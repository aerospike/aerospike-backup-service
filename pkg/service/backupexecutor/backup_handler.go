package backupexecutor

import (
	"context"

	"github.com/aerospike/backup-go/models"
)

// BackupHandler interface defines the contract for backup operation results.
type BackupHandler interface {
	// GetStats returns the statistics of the backup job.
	GetStats() *models.BackupStats
	// Wait waits for the backup job to complete and returns an error if the job failed.
	Wait(context.Context) error
	// GetMetrics returns the performance metrics of the backup job.
	GetMetrics() *models.Metrics
}
