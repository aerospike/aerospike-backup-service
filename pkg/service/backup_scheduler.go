package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/internal/log"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/reugn/go-quartz/logger"
	"github.com/reugn/go-quartz/quartz"
)

type (
	quartzGroup string
	jobType     string
)

const (
	quartzGroupAdHoc     quartzGroup = "ad-hoc"
	quartzGroupScheduled quartzGroup = "scheduled"

	jobTypeFull         jobType = "full"
	jobTypeIncremental  jobType = "incremental"
	minAdHocBackupDelay         = 50 * time.Millisecond
)

func (j jobType) String() string {
	if j == jobTypeFull {
		return "Full backup"
	}

	return "Incremental backup"
}

type Scheduler interface {
	// ScheduleJob schedules a backup job with the given trigger.
	ScheduleJob(jobDetail *quartz.JobDetail, trigger quartz.Trigger) error
}

// AdHocScheduler schedules one-off full or incremental backup jobs on demand.
type AdHocScheduler interface {
	// TriggerAdHocFullBackup schedules one full backup run for routineName after delay.
	TriggerAdHocFullBackup(routine *model.BackupRoutine, delay time.Duration) error
	// TriggerAdHocIncrementalBackup schedules one incremental backup run for routineName after delay.
	TriggerAdHocIncrementalBackup(routine *model.BackupRoutine, delay time.Duration) error
}

// adHocSchedulerImpl resolves routines and builds ad-hoc jobs at trigger time.
type adHocSchedulerImpl struct {
	scheduler  quartz.Scheduler
	components *BackupComponents
}

var _ AdHocScheduler = &adHocSchedulerImpl{}

// NewAdHocScheduler creates an ad-hoc scheduler backed by quartz and current config.
func NewAdHocScheduler(
	scheduler quartz.Scheduler,
	components *BackupComponents,
) AdHocScheduler {
	return &adHocSchedulerImpl{
		scheduler:  scheduler,
		components: components,
	}
}

// TriggerAdHocFullBackup schedules a one-off full backup for routineName.
func (s *adHocSchedulerImpl) TriggerAdHocFullBackup(routine *model.BackupRoutine, delay time.Duration) error {
	return s.triggerAdHocBackup(routine, delay, jobTypeFull)
}

// TriggerAdHocIncrementalBackup schedules a one-off incremental backup for routineName.
func (s *adHocSchedulerImpl) TriggerAdHocIncrementalBackup(routine *model.BackupRoutine, delay time.Duration) error {
	return s.triggerAdHocBackup(routine, delay, jobTypeIncremental)
}

// triggerAdHocBackup resolves routine, creates a fresh job, and schedules it once.
func (s *adHocSchedulerImpl) triggerAdHocBackup(
	routine *model.BackupRoutine,
	delay time.Duration,
	jobType jobType,
) error {
	runner := newOrchestrator(routine.Copy(), s.components) // orchestrator will work with its own copy
	jobDetail := quartz.NewJobDetail(newBackupJob(runner, jobType, routine.Name), adhocKey(routine.Name))

	return s.scheduler.ScheduleJob(jobDetail, quartz.NewRunOnceTrigger(max(delay, minAdHocBackupDelay)))
}

// NewScheduler creates a new quartz.Scheduler.
func NewScheduler(ctx context.Context, appLogger *slog.Logger) (quartz.Scheduler, error) {
	warnOnlyLogger := log.NewMinLevelLogger(appLogger, slog.LevelWarn)
	scheduler, err := quartz.NewStdScheduler(
		quartz.WithOutdatedThreshold(time.Second),
		quartz.WithLogger(logger.NewSlogLogger(ctx, warnOnlyLogger)),
		quartz.WithJobMetadata(),
	)
	return scheduler, err
}

// scheduleRoutines schedules provided routines.
func scheduleRoutines(
	scheduler Scheduler,
	routines []*model.BackupRoutine, components *BackupComponents,
) error {
	var errs error

	for _, routine := range routines {
		if routine.Disabled {
			slog.Debug("Skipping disabled routine", attr.Routine(routine.Name))
			continue
		}

		errs = errors.Join(errs, scheduleRoutineBackups(scheduler, routine.Copy(), components))
	}

	return errs
}

func scheduleRoutineBackups(
	scheduler Scheduler,
	routine *model.BackupRoutine,
	components *BackupComponents,
) error {
	runner := newOrchestrator(routine, components)

	fullJob := quartz.NewJobDetail(
		newBackupJob(runner, jobTypeFull, routine.Name),
		jobKey(routine.Name, jobTypeFull),
	)
	if err := schedule(scheduler, routine.IntervalCron, fullJob); err != nil {
		return fmt.Errorf("failed to schedule full backup: %w", err)
	}

	if len(routine.IncrIntervalCron) == 0 {
		// Incremental scheduling is optional and skipped when cron is not configured.
		return nil
	}

	incrementalJob := quartz.NewJobDetail(
		newBackupJob(runner, jobTypeIncremental, routine.Name),
		jobKey(routine.Name, jobTypeIncremental),
	)
	if err := schedule(scheduler, routine.IncrIntervalCron, incrementalJob); err != nil {
		return fmt.Errorf("failed to schedule incremental backup: %w", err)
	}

	return nil
}

func schedule(scheduler Scheduler, interval string, jobDetail *quartz.JobDetail) error {
	cronTrigger, err := quartz.NewCronTrigger(interval)
	if err != nil {
		return err
	}

	fireTime, err := cronTrigger.NextFireTime(time.Now().UnixNano())
	if err != nil {
		return err
	}
	if job, ok := jobDetail.Job().(*backupJob); ok {
		job.logger.Info("Schedule", slog.Any("nextRun", time.Unix(0, fireTime)))
	} else {
		slog.Warn("Unexpected job type", slog.Any("job", jobDetail))
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
