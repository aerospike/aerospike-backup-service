package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/reugn/go-quartz/logger"
	"github.com/reugn/go-quartz/quartz"
)

type (
	quartzGroup string
	jobType     string
)

const (
	quartzGroupAdHoc     quartzGroup = "ad-hoc"
	quartzGroupScheduled quartzGroup = "scheduled"

	jobTypeFull        jobType = "full"
	jobTypeIncremental jobType = "incremental"
)

type Scheduler interface {
	ScheduleJob(jobDetail *quartz.JobDetail, trigger quartz.Trigger) error
}

var jobStore = collections.NewSafeMap[string, *quartz.JobDetail]()

// NewAdHocFullBackupJobForRoutine returns a new full backup job for the routine name.
func NewAdHocFullBackupJobForRoutine(routineName string) *quartz.JobDetail {
	key := jobKey(routineName, jobTypeFull).String()
	job, found := jobStore.Load(key)
	if !found {
		return nil
	}

	jobKey := adhocKey(routineName)

	return quartz.NewJobDetail(job.Job(), jobKey)
}

// NewScheduler creates a new quartz.Scheduler.
func NewScheduler(ctx context.Context, appLogger *slog.Logger) (quartz.Scheduler, error) {
	scheduler, err := quartz.NewStdScheduler(
		quartz.WithOutdatedThreshold(time.Second),
		quartz.WithLogger(logger.NewSlogLogger(ctx, appLogger)),
		quartz.WithJobMetadata(),
	)
	return scheduler, err
}

// scheduleRoutines schedules the given handlers using the scheduler.
func scheduleRoutines(
	scheduler Scheduler, config *model.Config, components *BackupComponents,
) error {
	newJobs := map[string]*quartz.JobDetail{}
	var errs error
	for routineName, routine := range config.Routines() {
		if routine.Disabled {
			slog.Debug("Skipping disabled routine", attr.Routine(routineName))
			continue
		}

		runner := newOrchestrator(routineName, config, components)
		// schedule a full backup job for the routine
		job, err := scheduleFullBackup(scheduler, runner, routine.IntervalCron, routineName)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to schedule full backup: %w", err))
			continue
		}
		newJobs[job.JobKey().String()] = job

		// schedule an incremental backup job for the routine
		if err = scheduleIncrementalBackup(scheduler, runner, routine.IncrIntervalCron, routineName); err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to schedule incremental backup: %w", err))
		}
	}

	jobStore.ReplaceContent(newJobs)

	return errs
}

func scheduleFullBackup(
	scheduler Scheduler, runner backupRunner, interval string, routineName string,
) (*quartz.JobDetail, error) {
	job := createJobDetail(runner, routineName, jobTypeFull)
	return job, schedule(scheduler, interval, job)
}

func scheduleIncrementalBackup(
	scheduler Scheduler, runner backupRunner, interval string, routineName string,
) error {
	if len(interval) == 0 { // no need to schedule if there is no interval set
		return nil
	}

	job := createJobDetail(runner, routineName, jobTypeIncremental)
	return schedule(scheduler, interval, job)
}

func createJobDetail(runner backupRunner, routineName string, jobType jobType) *quartz.JobDetail {
	job := newBackupJob(runner, jobType, routineName)
	return quartz.NewJobDetail(job, jobKey(routineName, jobType))
}

func schedule(scheduler Scheduler, interval string, jobDetail *quartz.JobDetail) error {
	cronTrigger, err := quartz.NewCronTrigger(interval)
	if err != nil {
		return err
	}

	fireTime, err := cronTrigger.NextFireTime(time.Now().UnixNano())
	if err != nil {
		return err
	}
	if job, ok := jobDetail.Job().(*backupJob); ok {
		job.logger.Info("Schedule", slog.Any("nextRun", time.Unix(0, fireTime)))
	} else {
		slog.Warn("Unexpected job type", slog.Any("job", jobDetail))
	}

	return scheduler.ScheduleJob(jobDetail, cronTrigger)
}

func jobKey(routineName string, jobType jobType) *quartz.JobKey {
	jobName := fmt.Sprintf("%s-%s", routineName, jobType)
	return quartz.NewJobKeyWithGroup(jobName, string(quartzGroupScheduled))
}

func adhocKey(name string) *quartz.JobKey {
	jobName := fmt.Sprintf("%s-adhoc-%d", name, time.Now().UnixMilli())
	return quartz.NewJobKeyWithGroup(jobName, string(quartzGroupAdHoc))
}
