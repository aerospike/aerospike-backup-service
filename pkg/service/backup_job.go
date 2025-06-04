package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	"github.com/reugn/go-quartz/quartz"
)

// backupRunner defines an interface for running backups.
type backupRunner interface {
	// runFullBackup starts a full backup operation.
	runFullBackup(ctx context.Context, now time.Time)
	// runIncrementalBackup starts an incremental backup operation.
	runIncrementalBackup(ctx context.Context, now time.Time)
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
	if j.isRunning.CompareAndSwap(false, true) {
		defer j.isRunning.Store(false)
		switch j.jobType {
		case jobTypeFull:
			j.runner.runFullBackup(ctx, util.NowWithZeroMillis())
		case jobTypeIncremental:
			j.runner.runIncrementalBackup(ctx, util.NowWithZeroMillis())
		default:
			j.logger.Error("Unsupported backup type")
		}
		return nil
	}

	j.logger.Debug("Backup is currently in progress, skipping it")
	observeBackupEvent(j.routineName, j.jobType, BackupOutcomeSkipped, 0)

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
		logger:      slog.Default().With(slog.String("routine", routineName), slog.Any("type", jobType)),
	}
}
