package service

import (
	"context"
	"time"

	"github.com/aerospike/backup/pkg/model"
	"github.com/reugn/go-quartz/job"
	"github.com/reugn/go-quartz/logger"
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
		handler := newBackupHandler(config, routineName, backends[routineName])
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

		// schedule incrmental backup job for the routine
		if routine.IncrIntervalCron == "" {
			logger.Debugf("No incremental backup configured", "routine", routineName)
			return nil
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
