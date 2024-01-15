package service

import (
	"context"
	"time"

	"github.com/aerospike/backup/pkg/model"
	"github.com/reugn/go-quartz/job"
	"github.com/reugn/go-quartz/quartz"
)

// RunSchedule builds a list of BackupSchedulers according to
// the given configuration.
func RunSchedule(ctx context.Context, config *model.Config, backends map[string]BackupBackend) {
	sched := quartz.NewStdScheduler()
	sched.Start(ctx)

	for routineName := range config.BackupRoutines {
		handler := newBackupHandler(config, routineName, backends[routineName])
		fullCronTrigger, _ := quartz.NewCronTrigger("1/10 * * * * *") // every 10 sec
		fullJob := job.NewFunctionJob(func(ctx context.Context) (int, error) {
			handler.runFullBackup(time.Now())
			return 0, nil
		})
		fullJobDetail := quartz.NewJobDetail(fullJob, quartz.NewJobKeyWithGroup(routineName, "full"))
		sched.ScheduleJob(fullJobDetail, fullCronTrigger)
		incrCronTrigger, _ := quartz.NewCronTrigger("1/2 * * * * *") // every 2 sec
		incJob := job.NewFunctionJob(func(ctx context.Context) (int, error) {
			handler.runIncrementalBackup(time.Now())
			return 0, nil
		})
		incJobDetail := quartz.NewJobDetail(incJob, quartz.NewJobKeyWithGroup(routineName, "inc"))
		sched.ScheduleJob(incJobDetail, incrCronTrigger)
	}
}
