package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/aerospike/backup/pkg/model"
	"github.com/reugn/go-quartz/quartz"
)

const (
	QuartzGroupBackupFull        = "full"
	QuartzGroupBackupIncremental = "incremental"
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

// GetFullBackupJobByRoutineName returns the scheduled full backup job by routine name.
func GetFullBackupJobByRoutineName(name string) *quartz.JobDetail {
	jobStore.Lock()
	jobStore.Unlock()
	key := quartz.NewJobKeyWithGroup(name, QuartzGroupBackupFull).String()
	return jobStore.jobs[key]
}

// ScheduleBackup creates a new quartz.Scheduler, schedules all the configured backup jobs,
// starts and returns the scheduler.
func ScheduleBackup(ctx context.Context, config *model.Config, backends map[string]BackupBackend,
) (quartz.Scheduler, error) {
	scheduler := quartz.NewStdScheduler()
	scheduler.Start(ctx)

	for routineName, routine := range config.BackupRoutines {
		backend := backends[routineName]
		handler := newBackupHandler(config, routineName, backend)

		// schedule full backup job for the routine
		if err := scheduleFullBackup(scheduler, handler, routine, routineName); err != nil {
			return nil, err
		}

		if routine.IncrIntervalCron != "" {
			// schedule incremental backup job for the routine
			if err := scheduleIncrementalBackup(scheduler, handler, routine, routineName); err != nil {
				return nil, err
			}
		}
	}
	return scheduler, nil
}

func scheduleFullBackup(scheduler quartz.Scheduler, handler *BackupHandler,
	routine *model.BackupRoutine, routineName string) error {
	fullCronTrigger, err := quartz.NewCronTrigger(routine.IntervalCron)
	if err != nil {
		return err
	}
	fullJob := newBackupJob(handler, QuartzGroupBackupFull)
	fullJobDetail := quartz.NewJobDetail(
		fullJob,
		quartz.NewJobKeyWithGroup(routineName, QuartzGroupBackupFull),
	)
	if err = scheduler.ScheduleJob(fullJobDetail, fullCronTrigger); err != nil {
		return err
	}
	jobStore.put(fullJobDetail.JobKey().String(), fullJobDetail)
	if needToRunFullBackupNow(handler) {
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

func scheduleIncrementalBackup(scheduler quartz.Scheduler, handler *BackupHandler,
	routine *model.BackupRoutine, routineName string) error {
	incrCronTrigger, err := quartz.NewCronTrigger(routine.IncrIntervalCron)
	if err != nil {
		return err
	}
	incrementalJob := newBackupJob(handler, QuartzGroupBackupIncremental)
	incrJobDetail := quartz.NewJobDetail(
		incrementalJob,
		quartz.NewJobKeyWithGroup(routineName, QuartzGroupBackupIncremental),
	)
	if err = scheduler.ScheduleJob(incrJobDetail, incrCronTrigger); err != nil {
		return err
	}
	jobStore.put(incrJobDetail.JobKey().String(), incrJobDetail)
	return nil
}

func needToRunFullBackupNow(backupHandler *BackupHandler) bool {
	return backupHandler.state.LastFullRun.Equal(time.Time{})
}
