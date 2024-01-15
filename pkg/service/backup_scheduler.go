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

// RunSchedule builds a list of BackupSchedulers according to
// the given configuration.
func RunSchedule(ctx context.Context, config *model.Config, backends map[string]BackupBackend) error {
	scheduler := quartz.NewStdScheduler()
	scheduler.Start(ctx)

	for routineName, routine := range config.BackupRoutines {
		backend := backends[routineName]

		handler := newBackupHandler(config, routineName, backend)
		// schedule full backup job for the routine
		fullCronTrigger, err := quartz.NewCronTrigger(routine.IntervalCron)
		if err != nil {
			return err
		}
		fullJob := job.NewFunctionJob(func(ctx context.Context) (int, error) {
			handler.runFullBackup(time.Now())
			return 0, nil
		})
		fullJobDetail := quartz.NewJobDetail(
			fullJob,
			quartz.NewJobKeyWithGroup(routineName, quartzGroupBackupFull),
		)
		err = scheduler.ScheduleJob(fullJobDetail, fullCronTrigger)
		if err != nil {
			return err
		}
		if needToRunFullBackupNow(backend) {
			slog.Debug("Schedule full backup once", "name", routineName)
			fullJobDetail := quartz.NewJobDetail(
				fullJob,
				quartz.NewJobKey(routineName),
			)

			err = scheduler.ScheduleJob(fullJobDetail, quartz.NewRunOnceTrigger(0))
			if err != nil {
				return err
			}
		}

		// schedule incremental backup job for the routine
		if routine.IncrIntervalCron == "" {
			slog.Debug("No incremental backup configured", "routine", routineName)
			continue
		}
		incrCronTrigger, err := quartz.NewCronTrigger(routine.IncrIntervalCron)
		if err != nil {
			return err
		}
		incJob := job.NewFunctionJob(func(ctx context.Context) (int, error) {
			handler.runIncrementalBackup(time.Now())
			return 0, nil
		})
		incrJobDetail := quartz.NewJobDetail(
			incJob,
			quartz.NewJobKeyWithGroup(routineName, quartzGroupBackupIncremental),
		)
		err = scheduler.ScheduleJob(incrJobDetail, incrCronTrigger)
		if err != nil {
			return err
		}
	}
	return nil
}

func needToRunFullBackupNow(backend BackupBackend) bool {
	return backend.readState().LastFullRun == (time.Time{})
}
