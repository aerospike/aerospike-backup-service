package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/timeutil"
)

var errBackupSkipped = errors.New("backup skipped")

// BackupOrchestrator runs one full or incremental backup of a routine, from admission
// through execution to completion handling and reporting.
type BackupOrchestrator interface {
	// Backup executes a full or incremental backup for the given routine snapshot.
	Backup(ctx context.Context, routine *model.BackupRoutine, now time.Time, backupType model.BackupType)
}

// backupOrchestrator runs backup operations for a routine.
type backupOrchestrator struct {
	registry          backupRunCoordinator
	completionHandler BackupCompletionHandler
	outcomeReporter   BackupReporter
	startController   StartController
	routineRunner     RoutineBackupRunner
}

var _ BackupOrchestrator = (*backupOrchestrator)(nil)

// NewBackupOrchestrator returns a BackupOrchestrator.
func NewBackupOrchestrator(
	registry backupRunCoordinator,
	completionHandler BackupCompletionHandler,
	reporter BackupReporter,
	startController StartController,
	backupRunner RoutineBackupRunner,
) BackupOrchestrator {
	return &backupOrchestrator{
		registry:          registry,
		completionHandler: completionHandler,
		outcomeReporter:   reporter,
		startController:   startController,
		routineRunner:     backupRunner,
	}
}

// Backup executes a full or incremental backup for the given routine snapshot.
func (p *backupOrchestrator) Backup(
	ctx context.Context,
	routine *model.BackupRoutine,
	now time.Time,
	backupType model.BackupType,
) {
	logger := slog.With(attr.Routine(routine.Name))
	release, err := p.startController.TryStart(routine, now, backupType)
	if err != nil {
		p.outcomeReporter.Report(routine.Name, backupType, now, 0, err, logger)
		return
	}
	defer release()

	duration, err := timeutil.MeasureDuration(func() error {
		return p.runBackupInternal(ctx, routine, now, backupType, logger)
	})

	p.outcomeReporter.Report(routine.Name, backupType, now, duration, err, logger)
}

// runBackupInternal starts namespace backups, registers the aggregate handler, and runs completion hooks.
func (p *backupOrchestrator) runBackupInternal(
	ctx context.Context,
	routine *model.BackupRoutine,
	now time.Time,
	backupType model.BackupType,
	logger *slog.Logger,
) error {
	logger.Info(string(backupType)+" backup started", slog.Time("now", now))

	runSpec := model.BackupRunSpec{
		Type:       backupType,
		StartTime:  now,
		TimeBounds: p.createTimeBounds(backupType, now, routine),
	}
	backupHandler, err := p.routineRunner.Run(ctx, routine, runSpec, logger)
	if err != nil {
		return err
	}
	p.registry.register(routine.Name, backupType, backupHandler)

	if err = backupHandler.Wait(ctx); err != nil {
		p.completionHandler.OnFailure(routine, backupType)
		return fmt.Errorf("%s backup failed: %w", backupType, err)
	}

	p.completionHandler.OnSuccess(ctx, routine, backupType, now, logger)

	return nil
}

// createTimeBounds derives incremental from-time and optional sealed to-time for this run.
func (p *backupOrchestrator) createTimeBounds(
	backupType model.BackupType,
	now time.Time,
	routine *model.BackupRoutine,
) model.TimeBounds {
	var (
		fromTime *time.Time
		toTime   *time.Time
	)

	if backupType == model.BackupTypeIncremental {
		fromTime = p.registry.GetRoutineState(routine).LastRunTime.LatestRun()
	}

	if routine.BackupPolicy.IsSealedOrDefault() {
		toTime = &now
	}

	return model.TimeBounds{FromTime: fromTime, ToTime: toTime}
}
