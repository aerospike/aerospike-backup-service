package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
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
	sync.Mutex
	jobs map[string]*quartz.JobDetail
}

func (b *backupJobs) put(key string, value *quartz.JobDetail) {
	b.Lock()
	defer b.Unlock()
	b.jobs[key] = value
}

// NewAdHocFullBackupJobForRoutine returns a new full backup job for the routine name.
func NewAdHocFullBackupJobForRoutine(routineName string) *quartz.JobDetail {
	jobStore.Lock()
	defer jobStore.Unlock()

	key := fullJobKey(routineName).String()
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
	scheduler Scheduler, config *model.Config, handlers BackupHandlerHolder,
) error {
	var errs error
	for routineName, routine := range config.BackupRoutines {
		if routine.Disabled {
			continue
		}
		handler := handlers[routineName]

		// schedule a full backup job for the routine
		if err := scheduleFullBackup(scheduler, handler, routine.IntervalCron, routineName); err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to schedule full backup: %w", err))
			continue
		}

		if routine.IncrIntervalCron != "" {
			// schedule an incremental backup job for the routine
			if err := scheduleIncrementalBackup(scheduler, handler, routine.IncrIntervalCron, routineName); err != nil {
				errs = errors.Join(errs, fmt.Errorf("failed to schedule incremental backup: %w", err))
				continue
			}
		}
	}
	return errs
}

func scheduleFullBackup(
	scheduler Scheduler, handler backupRunner, interval string, routineName string,
) error {
	fullCronTrigger, err := quartz.NewCronTrigger(interval)
	if err != nil {
		return err
	}

	fullJob := newBackupJob(handler, jobTypeFull, routineName)
	fullJobDetail := quartz.NewJobDetail(fullJob, fullJobKey(routineName))
	jobStore.put(fullJobDetail.JobKey().String(), fullJobDetail)

	return scheduler.ScheduleJob(fullJobDetail, fullCronTrigger)
}

func scheduleIncrementalBackup(
	scheduler Scheduler, handler backupRunner, interval string, routineName string,
) error {
	incrCronTrigger, err := quartz.NewCronTrigger(interval)
	if err != nil {
		return err
	}

	incrementalJob := newBackupJob(handler, jobTypeIncremental, routineName)
	incrJobDetail := quartz.NewJobDetail(
		incrementalJob,
		incrJobKey(routineName),
	)
	jobStore.put(incrJobDetail.JobKey().String(), incrJobDetail)

	return scheduler.ScheduleJob(incrJobDetail, incrCronTrigger)
}

func incrJobKey(routineName string) *quartz.JobKey {
	jobName := fmt.Sprintf("%s-%s", routineName, jobTypeIncremental)
	return quartz.NewJobKeyWithGroup(jobName, string(quartzGroupScheduled))
}

func fullJobKey(routineName string) *quartz.JobKey {
	jobName := fmt.Sprintf("%s-%s", routineName, jobTypeFull)
	return quartz.NewJobKeyWithGroup(jobName, string(quartzGroupScheduled))
}

func adhocKey(name string) *quartz.JobKey {
	jobName := fmt.Sprintf("%s-adhoc-%d", name, time.Now().UnixMilli())
	return quartz.NewJobKeyWithGroup(jobName, string(quartzGroupAdHoc))
}
