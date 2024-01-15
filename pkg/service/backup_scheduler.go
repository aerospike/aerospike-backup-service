package service

import (
	"context"
	"github.com/reugn/go-quartz/quartz"
	"time"

	"github.com/aerospike/backup/pkg/model"
	"github.com/reugn/go-quartz/job"
)

// RunSchedule builds a list of BackupSchedulers according to
// the given configuration.
func RunSchedule(ctx context.Context, config *model.Config, backends map[string]BackupBackend) error {
	scheduler := quartz.NewStdScheduler()
	scheduler.Start(ctx)

	for routineName, routine := range config.BackupRoutines {
		handler := newBackupHandler(config, routineName, backends[routineName])
		fullCronTrigger, err := quartz.NewCronTrigger(routine.IntervalCron)
		if err != nil {
			return err
		}
		fullJob := job.NewFunctionJob(func(ctx context.Context) (int, error) {
			handler.runFullBackup(time.Now())
			return 0, nil
		})
		fullJobDetail := quartz.NewJobDetail(fullJob, quartz.NewJobKeyWithGroup(routineName, "full"))
		err = scheduler.ScheduleJob(fullJobDetail, fullCronTrigger)
		if err != nil {
			return err
		}
		incrCronTrigger, _ := quartz.NewCronTrigger(routine.IncrIntervalCron)
		incJob := job.NewFunctionJob(func(ctx context.Context) (int, error) {
			handler.runIncrementalBackup(time.Now())
			return 0, nil
		})
		incrJobDetail := quartz.NewJobDetail(incJob, quartz.NewJobKeyWithGroup(routineName, "inc"))
		err = scheduler.ScheduleJob(incrJobDetail, incrCronTrigger)
		if err != nil {
			return err
		}
	}
	return nil
}
