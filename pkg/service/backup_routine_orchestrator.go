package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/backupexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/timeutil"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/pkg/asinfo"
)

var (
	errBackupSkipped   = errors.New("full backup skipped")
	nonRetryableErrors = []error{asinfo.ErrNoNode}
)

// BackupRoutineOrchestrator orchestrates the execution of a single backup routine (both full and incremental).
// It manages all necessary preparations, executes the backup process, handles post-processing, and updates metrics.
type BackupRoutineOrchestrator struct {
	logger            *slog.Logger
	runner            *BackupNamespaceRunner
	routine           *model.BackupRoutine
	clientManager     aerospike.ClientManager
	registry          RunningBackupsRegistry
	completionHandler BackupCompletionHandler

	fullBackupLock sync.Mutex
}

var _ backupRunner = (*BackupRoutineOrchestrator)(nil)

// BackupComponents holds all components required to execute a backup routine.
type BackupComponents struct {
	clientManager     aerospike.ClientManager // Retrieves the Aerospike client before starting the backup.
	backupExecutor    backupexecutor.Backup   // Executes the backup using the backup-go library.
	registry          RunningBackupsRegistry  // Stores the backup handler during execution.
	completionHandler BackupCompletionHandler // Runs post-success/failure actions (registry, retention, cluster config).
	backendService    BackupWriter            // Writes backup metadata and deletes created files on failure.
}

func NewBackupComponents(
	clientManager aerospike.ClientManager,
	backupExecutor backupexecutor.Backup,
	registry RunningBackupsRegistry,
	completionHandler BackupCompletionHandler,
	backendService BackupWriter,
) *BackupComponents {
	return &BackupComponents{
		clientManager:     clientManager,
		backupExecutor:    backupExecutor,
		registry:          registry,
		completionHandler: completionHandler,
		backendService:    backendService,
	}
}

func newOrchestrator(
	routine *model.BackupRoutine,
	h *BackupComponents,
	pathService PathService,
) *BackupRoutineOrchestrator {
	logger := slog.With(attr.Routine(routine.Name))
	retry := newRetryExecutor(*routine.BackupPolicy.GetRetryPolicyOrDefault(), logger, nonRetryableErrors...)
	runner := NewBackupNamespaceRunner(routine, h.backupExecutor, retry, h.backendService, logger, pathService)
	return &BackupRoutineOrchestrator{
		routine:           routine,
		runner:            runner,
		clientManager:     h.clientManager,
		registry:          h.registry,
		completionHandler: h.completionHandler,
		logger:            logger,
	}
}

func (h *BackupRoutineOrchestrator) runFullBackup(ctx context.Context, now time.Time) {
	duration, err := timeutil.MeasureDuration(func() error {
		return h.runFullBackupInternal(ctx, now)
	})

	h.processBackupError(jobTypeFull, duration, err)
}

func (h *BackupRoutineOrchestrator) runFullBackupInternal(ctx context.Context, now time.Time) error {
	h.fullBackupLock.Lock()
	h.logger.Info("Full backup started", slog.Time("now", now))

	if h.skipFullBackup() {
		h.fullBackupLock.Unlock()
		return errBackupSkipped
	}

	client, namespaces, err := h.prepareCluster(ctx)
	if err != nil {
		h.fullBackupLock.Unlock()
		return err
	}
	defer h.clientManager.Close(client)

	timeBounds := h.createTimeBounds(jobTypeFull, now)
	backupHandler := startNamespacesBackup(ctx, h.runner, client, namespaces, timeBounds, now, h.routine, jobTypeFull)

	h.registry.register(h.routine.Name, jobTypeFull, backupHandler)
	// The lock must be held until the backup is registered.
	h.fullBackupLock.Unlock()

	if err = backupHandler.Wait(ctx); err != nil {
		h.completionHandler.OnFailure(h.routine, jobTypeFull)
		return fmt.Errorf("backup failed: %w", err)
	}

	h.completionHandler.OnSuccess(ctx, h.routine, jobTypeFull, now, h.logger)

	return nil
}

func (h *BackupRoutineOrchestrator) skipFullBackup() bool {
	currentStat := h.registry.GetRoutineState(h.routine)
	if currentStat.Full != nil {
		h.logger.Info("Full backup is currently in progress, skipping another full backup")
		return true
	}

	return false
}

func (h *BackupRoutineOrchestrator) prepareCluster(ctx context.Context) (aerospike.Client, []string, error) {
	client, err := h.clientManager.GetClient(ctx, h.routine.SourceCluster, h.logger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get backup client: %w", err)
	}

	namespaces, err := h.resolveNamespaces(ctx, h.routine.Namespaces, client.InfoClient())
	if err != nil {
		h.clientManager.Close(client)
		return nil, nil, fmt.Errorf("failed to retrieve namespaces from source cluster: %w", err)
	}

	return client, namespaces, nil
}

// resolveNamespaces returns the list of namespaces to back up.
// If `namespaces` is empty, it fetches all namespaces from the cluster via the provided client.
func (h *BackupRoutineOrchestrator) resolveNamespaces(
	ctx context.Context,
	namespaces []string,
	ig backup.InfoGetter,
) ([]string, error) {
	if len(namespaces) == 0 {
		return ig.GetNamespacesList(ctx)
	}

	return namespaces, nil
}

func (h *BackupRoutineOrchestrator) createTimeBounds(jobType jobType, now time.Time) model.TimeBounds {
	var (
		fromTime *time.Time
		toTime   *time.Time
	)

	if jobType == jobTypeIncremental {
		fromTime = h.registry.GetRoutineState(h.routine).LastRunTime.LatestRun()
	}

	if h.routine.BackupPolicy.IsSealedOrDefault() {
		toTime = &now
	}

	return model.TimeBounds{FromTime: fromTime, ToTime: toTime}
}

func (h *BackupRoutineOrchestrator) runIncrementalBackup(ctx context.Context, now time.Time) {
	h.logger.Info("Incremental backup started", slog.Time("now", now))

	if h.skipIncrementalBackup(now) {
		observeBackupEvent(h.routine.Name, jobTypeIncremental, BackupOutcomeSkip, 0)
		return
	}

	duration, err := timeutil.MeasureDuration(func() error {
		return h.runIncrementalBackupInternal(ctx, now)
	})

	h.processBackupError(jobTypeIncremental, duration, err)
}

func (h *BackupRoutineOrchestrator) processBackupError(backupType jobType, duration time.Duration, err error) {
	operation := backupType.String()

	if err == nil {
		h.logger.Debug(operation+" finished", slog.Duration("duration", duration))
		observeBackupEvent(h.routine.Name, backupType, BackupOutcomeSuccess, duration)
		return
	}

	if errors.Is(err, errBackupSkipped) {
		h.logger.Debug(operation + " skipped")
		observeBackupEvent(h.routine.Name, backupType, BackupOutcomeSkip, 0)
		return
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		h.logger.Info(operation + " context canceled")
		observeBackupEvent(h.routine.Name, backupType, BackupOutcomeCanceled, duration)
		return
	}

	var aerr *as.AerospikeError
	if errors.As(err, &aerr) {
		h.logger.Error(
			operation+" failed due to Aerospike error",
			slog.Int("resultCode", int(aerr.ResultCode)),
			attr.Error(err),
		)
	} else {
		h.logger.Error(operation+" failed", attr.Error(err))
	}

	observeBackupEvent(h.routine.Name, backupType, BackupOutcomeFailure, duration)
}

func (h *BackupRoutineOrchestrator) skipIncrementalBackup(now time.Time) bool {
	currentStat := h.registry.GetRoutineState(h.routine)

	// Skip if no initial full backup has been completed
	if currentStat.LastRunTime.NoFullBackup() {
		h.logger.Debug("Skipping incremental backup: initial full backup not yet completed")
		return true
	}

	// Concurrent incremental are only allowed when explicitly set (default is false).
	if h.routine.BackupPolicy.AllowConcurrentIncremental() {
		return false
	}

	switch {
	case currentStat.Full != nil:
		h.logger.Debug("Skipping incremental backup: full backup in progress")
		return true
	case currentStat.Incremental != nil:
		h.logger.Debug("Skipping incremental backup: another incremental backup in progress")
		return true
	case timeutil.IsCronFireTime(h.routine.IntervalCron, now):
		h.logger.Debug("Skipping incremental backup: full backup scheduled at same time")
		return true
	}

	return false
}

func (h *BackupRoutineOrchestrator) runIncrementalBackupInternal(ctx context.Context, now time.Time) error {
	client, namespaces, err := h.prepareCluster(ctx)
	if err != nil {
		return err
	}

	defer h.clientManager.Close(client)

	timeBounds := h.createTimeBounds(jobTypeIncremental, now)
	backupHandler := startNamespacesBackup(ctx,
		h.runner, client, namespaces, timeBounds, now, h.routine, jobTypeIncremental)
	h.registry.register(h.routine.Name, jobTypeIncremental, backupHandler)

	if err := backupHandler.Wait(ctx); err != nil {
		h.completionHandler.OnFailure(h.routine, jobTypeIncremental)
		return err
	}

	h.completionHandler.OnSuccess(ctx, h.routine, jobTypeIncremental, now, h.logger)

	return nil
}
