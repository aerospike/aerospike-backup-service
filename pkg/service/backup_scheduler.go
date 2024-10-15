package service

import (
	"context"
	"fmt"
	"log/slog"
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
func scheduleRoutines(scheduler quartz.Scheduler, config *model.Config,
	handlers BackupHandlerHolder) error {
	for routineName, routine := range config.BackupRoutines {
		handler := handlers[routineName]

		// schedule a full backup job for the routine
		if err := scheduleFullBackup(scheduler, handler, routine.IntervalCron, routineName); err != nil {
			return fmt.Errorf("failed to schedule full backup: %w", err)
		}

		if routine.IncrIntervalCron != "" {
			// schedule an incremental backup job for the routine
			if err := scheduleIncrementalBackup(scheduler, handler, routine.IncrIntervalCron, routineName); err != nil {
				return fmt.Errorf("failed to schedule incremental backup: %w", err)
			}
		}
	}
	return nil
}

func scheduleFullBackup(
	scheduler quartz.Scheduler, handler *BackupRoutineHandler, interval string, routineName string,
) error {
	fullCronTrigger, err := quartz.NewCronTrigger(interval)
	if err != nil {
		return err
	}

	fullJob := newBackupJob(handler, jobTypeFull)
	fullJobDetail := quartz.NewJobDetail(fullJob, fullJobKey(routineName))

	if err = scheduler.ScheduleJob(fullJobDetail, fullCronTrigger); err != nil {
		return err
	}

	jobStore.put(fullJobDetail.JobKey().String(), fullJobDetail)
	if needToRunFullBackupNow(handler.state.LastFullRun, fullCronTrigger) {
		slog.Debug("Schedule initial full backup", "name", routineName)
		fullJobDetail := quartz.NewJobDetail(
			fullJob,
			quartz.NewJobKey(routineName),
		)
		if err = scheduler.ScheduleJob(fullJobDetail, quartz.NewRunOnceTrigger(0)); err != nil {
			return err
		}
	}
	return nil
}

func scheduleIncrementalBackup(
	scheduler quartz.Scheduler, handler *BackupRoutineHandler, interval string, routineName string,
) error {
	incrCronTrigger, err := quartz.NewCronTrigger(interval)
	if err != nil {
		return err
	}

	incrementalJob := newBackupJob(handler, jobTypeIncremental)
	incrJobDetail := quartz.NewJobDetail(
		incrementalJob,
		incrJobKey(routineName),
	)

	if err = scheduler.ScheduleJob(incrJobDetail, incrCronTrigger); err != nil {
		return err
	}

	jobStore.put(incrJobDetail.JobKey().String(), incrJobDetail)
	return nil
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

func needToRunFullBackupNow(lastFullRun time.Time, trigger *quartz.CronTrigger) bool {
	if lastFullRun.Equal(time.Time{}) {
		return true // no previous run
	}

	fireTimeNano, err := trigger.NextFireTime(lastFullRun.UnixNano())
	if err != nil {
		return true // some error, run backup to be safe
	}
	if time.Unix(0, fireTimeNano).Before(time.Now()) {
		return true // next scheduled backup is in the past
	}

	return false
}
