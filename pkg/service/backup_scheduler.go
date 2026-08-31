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

type quartzGroup string

const (
	quartzGroupAdHoc     quartzGroup = "ad-hoc"
	quartzGroupScheduled quartzGroup = "scheduled"

	minAdHocBackupDelay = 50 * time.Millisecond
)

// JobScheduler is the part of quartz.Scheduler used to add and remove backup jobs.
type JobScheduler interface {
	// ScheduleJob registers jobDetail with the provided trigger.
	ScheduleJob(jobDetail *quartz.JobDetail, trigger quartz.Trigger) error
	// DeleteJob removes the job identified by key.
	DeleteJob(key *quartz.JobKey) error
}

// AdHocScheduler triggers a single backup outside the routine's cron schedule.
type AdHocScheduler interface {
	// TriggerAdHocFullBackup schedules one full backup run for the routine after delay.
	TriggerAdHocFullBackup(routine *model.BackupRoutine, delay time.Duration) error
	// TriggerAdHocIncrementalBackup schedules one incremental backup run for the routine after delay.
	TriggerAdHocIncrementalBackup(routine *model.BackupRoutine, delay time.Duration) error
}

// BackupScheduler wires Quartz to the backup orchestrator: periodic cron jobs, ad-hoc runs, and job deletion.
type BackupScheduler struct {
	scheduler JobScheduler
	// orchestrator runs each fired job (cron or ad-hoc) via [BackupOrchestrator.Backup].
	orchestrator BackupOrchestrator
}

var _ AdHocScheduler = (*BackupScheduler)(nil)

// NewBackupScheduler returns a BackupScheduler.
func NewBackupScheduler(scheduler JobScheduler, orchestrator BackupOrchestrator) *BackupScheduler {
	return &BackupScheduler{scheduler: scheduler, orchestrator: orchestrator}
}

// DeleteJob removes a scheduled job (e.g. when clearing periodic jobs on config change).
func (s *BackupScheduler) DeleteJob(key *quartz.JobKey) error {
	return s.scheduler.DeleteJob(key)
}

// ScheduleRoutines registers cron triggers for the given routines (full and optional incremental).
func (s *BackupScheduler) ScheduleRoutines(routines []*model.BackupRoutine) error {
	var errs error

	for _, routine := range routines {
		if routine.Disabled {
			slog.Debug("Skipping disabled routine", attr.Routine(routine.Name))
			continue
		}

		errs = errors.Join(errs, s.scheduleRoutineBackups(routine.Copy()))
	}

	return errs
}

// scheduleRoutineBackups registers the full cron job and, when configured, the incremental cron job for one routine.
func (s *BackupScheduler) scheduleRoutineBackups(routine *model.BackupRoutine) error {
	fullJob := quartz.NewJobDetail(
		newBackupJob(s.orchestrator, routine, model.BackupTypeFull),
		jobKey(routine.Name, model.BackupTypeFull),
	)
	if err := s.scheduleCronJob(routine.IntervalCron, routine.CronLocation(), fullJob); err != nil {
		return fmt.Errorf("failed to schedule full backup: %w", err)
	}

	if len(routine.IncrIntervalCron) == 0 {
		// Incremental scheduling is optional and skipped when cron is not configured.
		return nil
	}

	incrementalJob := quartz.NewJobDetail(
		newBackupJob(s.orchestrator, routine, model.BackupTypeIncremental),
		jobKey(routine.Name, model.BackupTypeIncremental),
	)
	if err := s.scheduleCronJob(routine.IncrIntervalCron, routine.CronLocation(), incrementalJob); err != nil {
		return fmt.Errorf("failed to schedule incremental backup: %w", err)
	}

	return nil
}

// scheduleCronJob attaches a cron trigger to jobDetail and schedules it on the underlying Quartz scheduler.
func (s *BackupScheduler) scheduleCronJob(interval string, loc *time.Location, jobDetail *quartz.JobDetail) error {
	cronTrigger, err := quartz.NewCronTriggerWithLoc(interval, loc)
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

	return s.scheduler.ScheduleJob(jobDetail, cronTrigger)
}

// TriggerAdHocFullBackup schedules a one-off full backup for routineName.
func (s *BackupScheduler) TriggerAdHocFullBackup(routine *model.BackupRoutine, delay time.Duration) error {
	return s.triggerAdHocBackup(routine, delay, model.BackupTypeFull)
}

// TriggerAdHocIncrementalBackup schedules a one-off incremental backup for routineName.
func (s *BackupScheduler) TriggerAdHocIncrementalBackup(routine *model.BackupRoutine, delay time.Duration) error {
	return s.triggerAdHocBackup(routine, delay, model.BackupTypeIncremental)
}

// triggerAdHocBackup schedules a single run-once job in the ad-hoc group after at least [minAdHocBackupDelay].
func (s *BackupScheduler) triggerAdHocBackup(
	routine *model.BackupRoutine,
	delay time.Duration,
	jt model.BackupType,
) error {
	jobDetail := quartz.NewJobDetail(newBackupJob(s.orchestrator, routine.Copy(), jt), adhocKey(routine.Name))

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

// jobKey returns the stable Quartz key for a periodic full or incremental job in the scheduled group.
func jobKey(routineName string, jt model.BackupType) *quartz.JobKey {
	jobName := fmt.Sprintf("%s-%s", routineName, jt)
	return quartz.NewJobKeyWithGroup(jobName, string(quartzGroupScheduled))
}

// adhocKey builds a unique job key for a one-off backup in the ad-hoc Quartz group.
func adhocKey(name string) *quartz.JobKey {
	jobName := fmt.Sprintf("%s-adhoc-%d", name, time.Now().UnixMilli())
	return quartz.NewJobKeyWithGroup(jobName, string(quartzGroupAdHoc))
}
