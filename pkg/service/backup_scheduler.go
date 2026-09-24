package service

import (
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/internal/log"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/reugn/go-quartz/matcher"
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
	// GetJobKeys returns the keys of the scheduled jobs that satisfy all matchers.
	GetJobKeys(matchers ...quartz.Matcher[quartz.ScheduledJob]) ([]*quartz.JobKey, error)
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
	// adHocSeq numbers ad-hoc jobs, so every trigger gets its own job key.
	adHocSeq atomic.Uint64
}

var _ AdHocScheduler = (*BackupScheduler)(nil)

// NewBackupScheduler returns a BackupScheduler.
func NewBackupScheduler(
	scheduler JobScheduler,
	orchestrator BackupOrchestrator,
) *BackupScheduler {
	return &BackupScheduler{
		scheduler:    scheduler,
		orchestrator: orchestrator,
	}
}

// DeleteJob removes a scheduled job (e.g. when clearing periodic jobs on config change).
func (s *BackupScheduler) DeleteJob(key *quartz.JobKey) error {
	return s.scheduler.DeleteJob(key)
}

// DeleteAdHocJobs removes the routine's pending ad-hoc jobs (e.g. when the routine was deleted).
func (s *BackupScheduler) DeleteAdHocJobs(routineName string) error {
	keys, err := s.scheduler.GetJobKeys(matcher.JobGroupEquals(adHocGroup(routineName)))
	if err != nil {
		return fmt.Errorf("failed to list ad-hoc jobs of routine %q: %w", routineName, err)
	}

	var errs error
	for _, key := range keys {
		// A job that fired meanwhile is gone already, and there is nothing left to delete.
		if err := s.scheduler.DeleteJob(key); err != nil && !errors.Is(err, quartz.ErrJobNotFound) {
			errs = errors.Join(errs, err)
		}
	}

	return errs
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

	if err := s.scheduleCronJob(routine.FullSchedule(), fullJob); err != nil {
		return fmt.Errorf("failed to schedule full backup: %w", err)
	}

	if !routine.HasIncrementalSchedule() {
		// Incremental scheduling is optional and skipped when cron is not configured.
		return nil
	}

	incrementalJob := quartz.NewJobDetail(
		newBackupJob(s.orchestrator, routine, model.BackupTypeIncremental),
		jobKey(routine.Name, model.BackupTypeIncremental),
	)
	if err := s.scheduleCronJob(routine.IncrementalSchedule(), incrementalJob); err != nil {
		return fmt.Errorf("failed to schedule incremental backup: %w", err)
	}

	return nil
}

// scheduleCronJob attaches a cron trigger to jobDetail and schedules it on the underlying Quartz scheduler.
func (s *BackupScheduler) scheduleCronJob(schedule model.Schedule, jobDetail *quartz.JobDetail) error {
	cronTrigger, err := quartz.NewCronTriggerWithLoc(schedule.Cron, schedule.Location)
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
	jobDetail := quartz.NewJobDetail(
		newBackupJob(s.orchestrator, routine.Copy(), jt),
		adhocKey(routine.Name, jt, s.adHocSeq.Add(1)),
	)

	return s.scheduler.ScheduleJob(jobDetail, quartz.NewRunOnceTrigger(max(delay, minAdHocBackupDelay)))
}

// NewScheduler creates a new quartz.Scheduler.
func NewScheduler(appLogger *slog.Logger) (quartz.Scheduler, error) {
	warnOnlyLogger := log.NewMinLevelLogger(appLogger, slog.LevelWarn)
	scheduler, err := quartz.NewStdScheduler(
		quartz.WithOutdatedThreshold(time.Second),
		quartz.WithLogger(schedulerLogger{warnOnlyLogger}),
		quartz.WithJobMetadata(),
	)

	return scheduler, err
}

// schedulerLogger adapts a slog.Logger to the logger quartz wants.
type schedulerLogger struct {
	*slog.Logger
}

func (l schedulerLogger) Trace(msg string, args ...any) {
	l.Debug(msg, args...)
}

// jobKey returns the stable Quartz key for a periodic full or incremental job in the scheduled group.
func jobKey(routineName string, jt model.BackupType) *quartz.JobKey {
	jobName := fmt.Sprintf("%s-%s", routineName, jt)
	return quartz.NewJobKeyWithGroup(jobName, string(quartzGroupScheduled))
}

// adhocKey builds the job key for a one-off backup in the routine's ad-hoc group. seq makes it
// unique among the jobs of one scheduler, however close together they are triggered.
func adhocKey(routineName string, jt model.BackupType, seq uint64) *quartz.JobKey {
	jobName := fmt.Sprintf("%s-adhoc-%s-%d", routineName, jt, seq)
	return quartz.NewJobKeyWithGroup(jobName, adHocGroup(routineName))
}

// adHocGroup is the Quartz group holding one routine's ad-hoc jobs, so they can be found by an
// exact match on the routine name rather than by parsing job names.
func adHocGroup(routineName string) string {
	return fmt.Sprintf("%s/%s", quartzGroupAdHoc, routineName)
}
