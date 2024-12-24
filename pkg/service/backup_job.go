package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/util"
	"github.com/reugn/go-quartz/quartz"
)

type backupRunner interface {
	runFullBackup(context.Context, time.Time)
	runIncrementalBackup(context.Context, time.Time)
	Cancel()
	CurrentStat() *model.CurrentBackups
}

// backupJob implements the quartz.Job interface.
type backupJob struct {
	handler     backupRunner
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
			j.handler.runFullBackup(ctx, util.NowWithZeroNanoseconds())
		case jobTypeIncremental:
			j.handler.runIncrementalBackup(ctx, util.NowWithZeroNanoseconds())
		default:
			j.logger.Error("Unsupported backup type")
		}
	} else {
		j.logger.Debug("Backup is currently in progress, skipping it")
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
	return fmt.Sprintf("%s %s backup job", j.routineName, j.jobType)
}

// newBackupJob creates a new backup job.
func newBackupJob(handler backupRunner, jobType jobType, routineName string) quartz.Job {
	return &backupJob{
		handler:     handler,
		jobType:     jobType,
		routineName: routineName,
		logger:      slog.Default().With(slog.String("routine", routineName), slog.Any("type", jobType)),
	}
}
