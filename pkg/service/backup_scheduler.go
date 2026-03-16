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

var ErrRoutineNotFound = errors.New("routine not found")

type AdHocScheduler interface {
	// TriggerAdHocFullBackup schedules one full backup run for routineName after delay.
	TriggerAdHocFullBackup(routineName string, delay time.Duration) error
	// TriggerAdHocIncrementalBackup schedules one incremental backup run for routineName after delay.
	TriggerAdHocIncrementalBackup(routineName string, delay time.Duration) error
}

type adHocSchedulerImpl struct {
	scheduler  quartz.Scheduler
	config     *model.Config
	components *BackupComponents
}

var _ AdHocScheduler = &adHocSchedulerImpl{}

func NewAdHocScheduler(
	scheduler quartz.Scheduler,
	config *model.Config,
	components *BackupComponents,
) AdHocScheduler {
	return &adHocSchedulerImpl{
		scheduler:  scheduler,
		config:     config,
		components: components,
	}
}

func (s *adHocSchedulerImpl) TriggerAdHocFullBackup(routineName string, delay time.Duration) error {
	return s.triggerAdHocBackup(routineName, delay, jobTypeFull)
}

func (s *adHocSchedulerImpl) TriggerAdHocIncrementalBackup(routineName string, delay time.Duration) error {
	return s.triggerAdHocBackup(routineName, delay, jobTypeIncremental)
}

func (s *adHocSchedulerImpl) triggerAdHocBackup(routineName string, delay time.Duration, jobType jobType) error {
	routine, found := s.config.Routine(routineName)
	if !found {
		return ErrRoutineNotFound
	}

	runner := newOrchestrator(routine.Copy(), s.components) // orchestrator will work with its own copy
	jobDetail := quartz.NewJobDetail(newBackupJob(runner, jobType, routine.Name), adhocKey(routine.Name))
	if jobDetail == nil {
		return errors.New("failed to create backup job")
	}

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

		runner := newOrchestrator(routine.Copy(), components) // orchestrator will work with its own copy
		// schedule a full backup job for the routine
		_, err := scheduleFullBackup(scheduler, runner, routine.IntervalCron, routine.Name)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to schedule full backup: %w", err))
			continue
		}

		// schedule an incremental backup job for the routine
		_, err = scheduleIncrementalBackup(scheduler, runner, routine.IncrIntervalCron, routine.Name)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to schedule incremental backup: %w", err))
			continue
		}
	}

	return errs
}

func scheduleFullBackup(
	scheduler Scheduler, runner backupRunner, interval string, routineName string,
) (*quartz.JobDetail, error) {
	job := createJobDetail(runner, routineName, jobTypeFull)
	return job, schedule(scheduler, interval, job)
}

func scheduleIncrementalBackup(
	scheduler Scheduler, runner backupRunner, interval string, routineName string,
) (*quartz.JobDetail, error) {
	job := createJobDetail(runner, routineName, jobTypeIncremental)
	if len(interval) == 0 { // no need to schedule if there is no interval set
		return job, nil
	}

	return job, schedule(scheduler, interval, job)
}

func createJobDetail(runner backupRunner, routineName string, jobType jobType) *quartz.JobDetail {
	job := newBackupJob(runner, jobType, routineName)
	return quartz.NewJobDetail(job, jobKey(routineName, jobType))
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
