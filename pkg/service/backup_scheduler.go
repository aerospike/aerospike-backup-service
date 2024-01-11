package service

import (
	"context"
	"github.com/reugn/go-quartz/quartz"
	"time"

	"github.com/aerospike/backup/pkg/model"
)

// RunSchedule builds a list of BackupSchedulers according to
// the given configuration.
func RunSchedule(ctx context.Context, config *model.Config, backends map[string]BackupBackend) {
	sched := quartz.NewStdScheduler()
	sched.Start(ctx)

	for routineName := range config.BackupRoutines {
		handler := newBackupHandler(config, routineName, backends[routineName])
		fullCronTrigger, _ := quartz.NewCronTrigger("1/10 * * * * *") // every 10 sec
		fullJob := quartz.NewFunctionJob(func(ctx context.Context) (int, error) {
			handler.runFullBackup(time.Now())
			return 0, nil
		})
		sched.ScheduleJob(ctx, fullJob, fullCronTrigger)
		incrCronTrigger, _ := quartz.NewCronTrigger("1/2 * * * * *") // every 2 sec
		incJob := quartz.NewFunctionJob(func(ctx context.Context) (int, error) {
			handler.runIncrementalBackup(time.Now())
			return 0, nil
		})
		sched.ScheduleJob(ctx, incJob, incrCronTrigger)
	}
}
