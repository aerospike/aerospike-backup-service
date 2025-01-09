package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
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

var jobStore = util.NewSafeMap[string, *quartz.JobDetail]()

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

// NewScheduler creates a new running quartz.Scheduler
func NewScheduler(ctx context.Context) quartz.Scheduler {
	scheduler := quartz.NewStdSchedulerWithOptions(quartz.StdSchedulerOptions{
		OutdatedThreshold: 1 * time.Second,
		RetryInterval:     100 * time.Millisecond,
	}, nil, nil)

	scheduler.Start(ctx)

	return scheduler
}

// scheduleRoutines schedules the given handlers using the scheduler.
func scheduleRoutines(
	scheduler Scheduler, routines map[string]*model.BackupRoutine, handlers BackupHandlerHolder,
) error {
	newJobs := map[string]*quartz.JobDetail{}
	var errs error
	for routineName, routine := range routines {
		if routine.Disabled {
			continue
		}
		handler, found := handlers.Load(routineName)
		if !found {
			errs = errors.Join(errs, fmt.Errorf("handler not found for routine %q", routineName))
			continue
		}

		// schedule a full backup job for the routine
		job, err := scheduleFullBackup(scheduler, handler, routine.IntervalCron, routineName)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to schedule full backup: %w", err))
			continue
		}
		newJobs[job.JobKey().String()] = job

		// schedule an incremental backup job for the routine
		if err := scheduleIncrementalBackup(scheduler, handler, routine.IncrIntervalCron, routineName); err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to schedule incremental backup: %w", err))
		}
	}

	jobStore.ReplaceContent(newJobs)
	return errs
}

func scheduleFullBackup(
	scheduler Scheduler, handler backupRunner, interval string, routineName string,
) (*quartz.JobDetail, error) {
	job := createJobDetail(handler, routineName, jobTypeFull)
	return job, schedule(scheduler, interval, job)
}

func scheduleIncrementalBackup(
	scheduler Scheduler, handler backupRunner, interval string, routineName string,
) error {
	if len(interval) == 0 { // no need to schedule if there is no interval set
		return nil
	}

	job := createJobDetail(handler, routineName, jobTypeIncremental)
	return schedule(scheduler, interval, job)
}

func createJobDetail(handler backupRunner, routineName string, jobType jobType) *quartz.JobDetail {
	job := newBackupJob(handler, jobType, routineName)
	return quartz.NewJobDetail(job, jobKey(routineName, jobType))
}

func schedule(scheduler Scheduler, interval string, jobDetail *quartz.JobDetail) error {
	cronTrigger, err := quartz.NewCronTrigger(interval)
	if err != nil {
		return err
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
