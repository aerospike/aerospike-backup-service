package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
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

var jobStore = &backupJobs{jobs: make(map[string]*quartz.JobDetail)}

type backupJobs struct {
	sync.RWMutex
	jobs map[string]*quartz.JobDetail
}

// NewAdHocFullBackupJobForRoutine returns a new full backup job for the routine name.
func NewAdHocFullBackupJobForRoutine(routineName string) *quartz.JobDetail {
	jobStore.RLock()
	defer jobStore.RUnlock()

	key := jobKey(routineName, jobTypeFull).String()
	job := jobStore.jobs[key]
	if job == nil {
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
	jobStore.Lock()
	defer jobStore.Unlock()
	clear(jobStore.jobs)

	var errs error
	for routineName, routine := range routines {
		if routine.Disabled {
			continue
		}
		handler, _ := handlers.Load(routineName)

		// schedule a full backup job for the routine
		if err := scheduleFullBackup(scheduler, handler, routine.IntervalCron, routineName); err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to schedule full backup: %w", err))
			continue
		}

		// schedule an incremental backup job for the routine
		if err := scheduleIncrementalBackup(scheduler, handler, routine.IncrIntervalCron, routineName); err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to schedule incremental backup: %w", err))
		}
	}

	return errs
}

func scheduleFullBackup(
	scheduler Scheduler, handler backupRunner, interval string, routineName string,
) error {
	job := createJobDetail(handler, routineName, jobTypeFull)
	key := job.JobKey().String()
	jobStore.jobs[key] = job
	return schedule(scheduler, interval, job)
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
