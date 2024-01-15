package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/aerospike/backup/pkg/model"
	"github.com/reugn/go-quartz/job"
	"github.com/reugn/go-quartz/quartz"
)

const (
	quartzGroupBackupFull        = "full"
	quartzGroupBackupIncremental = "incremental"
)

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
		} else {
			slog.Debug("No incremental backup configured", "routine", routineName)
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
	fullJob := job.NewFunctionJob(func(_ context.Context) (int, error) {
		handler.runFullBackup(time.Now())
		return 0, nil
	})
	fullJobDetail := quartz.NewJobDetail(
		fullJob,
		quartz.NewJobKeyWithGroup(routineName, quartzGroupBackupFull),
	)
	if err = scheduler.ScheduleJob(fullJobDetail, fullCronTrigger); err != nil {
		return err
	}
	if needToRunFullBackupNow(handler.backend) {
		slog.Debug("Schedule initial full backup", "routine", routineName)
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
	incJob := job.NewFunctionJob(func(_ context.Context) (int, error) {
		handler.runIncrementalBackup(time.Now())
		return 0, nil
	})
	incrJobDetail := quartz.NewJobDetail(
		incJob,
		quartz.NewJobKeyWithGroup(routineName, quartzGroupBackupIncremental),
	)
	return scheduler.ScheduleJob(incrJobDetail, incrCronTrigger)
}

func needToRunFullBackupNow(backend BackupBackend) bool {
	return backend.readState().LastFullRun == (time.Time{})
}
