package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/reugn/go-quartz/quartz"
)

// backupJob implements the quartz.Job interface.
type backupJob struct {
	handler   *BackupRoutineHandler
	jobType   jobType
	isRunning atomic.Bool
}

var _ quartz.Job = (*backupJob)(nil)

// Execute is called by a Scheduler when the Trigger associated with this job fires.
func (j *backupJob) Execute(ctx context.Context) error {
	logger := slog.Default().With(slog.String("routine", j.handler.routineName),
		slog.Any("type", j.jobType))

	if j.isRunning.CompareAndSwap(false, true) {
		defer j.isRunning.Store(false)
		switch j.jobType {
		case jobTypeFull:
			j.handler.runFullBackup(ctx, time.Now())
		case jobTypeIncremental:
			j.handler.runIncrementalBackup(ctx, time.Now())
		default:
			logger.Error("Unsupported backup type")
		}
	} else {
		logger.Debug("Backup is currently in progress, skipping it")
		incrementSkippedCounters(j.jobType)
	}

	return nil
}

func incrementSkippedCounters(jobType jobType) {
	switch jobType {
	case jobTypeFull:
		backupSkippedCounter.Inc()
	case jobTypeIncremental:
		incrBackupSkippedCounter.Inc()
	}
}

// Description returns the description of the backup job.
func (j *backupJob) Description() string {
	return fmt.Sprintf("%s %s backup job", j.handler.routineName, j.jobType)
}

// newBackupJob creates a new backup job.
func newBackupJob(handler *BackupRoutineHandler, jobType jobType) quartz.Job {
	return &backupJob{
		handler: handler,
		jobType: jobType,
	}
}
