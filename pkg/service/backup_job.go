package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/reugn/go-quartz/quartz"
)

// backupRunner defines an interface for running backups.
type backupRunner interface {
	// runBackup starts a backup operation for the requested backup type.
	runBackup(ctx context.Context, now time.Time, backupType jobType)
}

// backupJob implements the quartz.Job interface.
type backupJob struct {
	runner      backupRunner
	jobType     jobType
	isRunning   atomic.Bool
	routineName string
	logger      *slog.Logger
}

var _ quartz.Job = (*backupJob)(nil)

// Execute is called by a Scheduler when the Trigger associated with this job fires.
func (j *backupJob) Execute(ctx context.Context) error {
	jobMetadata, ok := ctx.Value(quartz.JobMetadataContextKey).(quartz.JobMetadata)
	if !ok {
		return errors.New("failed to retrieve job metadata from context. " +
			"Use quartz.WithJobMetadata() option when initializing the scheduler")
	}
	now := time.Unix(0, jobMetadata.RunTime).Truncate(time.Millisecond)

	if j.isRunning.CompareAndSwap(false, true) {
		defer j.isRunning.Store(false)
		j.runner.runBackup(ctx, now, j.jobType)
		return nil
	}

	j.logger.Debug("Backup is currently in progress, skipping it")
	observeBackupEvent(j.routineName, j.jobType, BackupOutcomeSkip, 0)

	return nil
}

// Description returns the description of the backup job.
func (j *backupJob) Description() string {
	return fmt.Sprintf("%s %s backup job", j.routineName, j.jobType)
}

// newBackupJob creates a new backup job.
func newBackupJob(runner backupRunner, jobType jobType, routineName string) quartz.Job {
	return &backupJob{
		runner:      runner,
		jobType:     jobType,
		routineName: routineName,
		logger:      slog.Default().With(attr.Routine(routineName), slog.Any("type", jobType)),
	}
}
