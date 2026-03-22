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

// BackupOrchestrator runs backup operations for a routine. It holds only shared dependencies;
// each invocation receives an immutable *model.BackupRoutine snapshot.
type BackupOrchestrator struct {
	registry          RunningBackupsRegistry
	completionHandler BackupCompletionHandler
	startController   StartController
	nsExec            *NamespaceBackupExecutor
}

// NewBackupOrchestrator builds a [BackupOrchestrator] from shared service dependencies.
func NewBackupOrchestrator(
	registry RunningBackupsRegistry,
	completionHandler BackupCompletionHandler,
	startController StartController,
	nsExec *NamespaceBackupExecutor,
) *BackupOrchestrator {
	return &BackupOrchestrator{
		registry:          registry,
		completionHandler: completionHandler,
		startController:   startController,
		nsExec:            nsExec,
	}
}

// RunBackup executes a full or incremental backup for the given routine snapshot.
func (p *BackupOrchestrator) RunBackup(
	ctx context.Context,
	routine *model.BackupRoutine,
	now time.Time,
	backupType model.BackupType,
) {
	logger := slog.With(attr.Routine(routine.Name))
	release, err := p.startController.TryStart(routine, now, backupType)
	if err != nil {
		reportBackupOutcome(routine.Name, backupType, 0, err, logger)
		return
	}
	defer release()

	duration, err := timeutil.MeasureDuration(func() error {
		return p.runBackupInternal(ctx, routine, now, backupType, logger)
	})

	reportBackupOutcome(routine.Name, backupType, duration, err, logger)
}

// runBackupInternal starts namespace backups, registers the aggregate handler, and runs completion hooks.
func (p *BackupOrchestrator) runBackupInternal(
	ctx context.Context,
	routine *model.BackupRoutine,
	now time.Time,
	backupType model.BackupType,
	logger *slog.Logger,
) error {
	logger.Info(backupType.String()+" backup started", slog.Time("now", now))

	runSpec := model.BackupRunSpec{
		Type:       backupType,
		StartTime:  now,
		TimeBounds: p.createTimeBounds(backupType, now, routine),
	}
	backupHandler, err := p.nsExec.StartBackup(ctx, logger, routine, runSpec)
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
func (p *BackupOrchestrator) createTimeBounds(
	jobType model.BackupType,
	now time.Time,
	routine *model.BackupRoutine,
) model.TimeBounds {
	var (
		fromTime *time.Time
		toTime   *time.Time
	)

	if jobType == model.BackupJobTypeIncremental {
		fromTime = p.registry.GetRoutineState(routine).LastRunTime.LatestRun()
	}

	if routine.BackupPolicy.IsSealedOrDefault() {
		toTime = &now
	}

	return model.TimeBounds{FromTime: fromTime, ToTime: toTime}
}
