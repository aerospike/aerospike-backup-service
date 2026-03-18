package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/backupexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/timeutil"
	"github.com/aerospike/backup-go/pkg/asinfo"
)

var (
	errBackupSkipped   = errors.New("backup skipped")
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
	startController   StartController
}

var _ backupRunner = (*BackupRoutineOrchestrator)(nil)

// BackupComponents holds all components required to execute a backup routine.
type BackupComponents struct {
	clientManager     aerospike.ClientManager // Retrieves the Aerospike client before starting the backup.
	backupExecutor    backupexecutor.Backup   // Executes the backup using the backup-go library.
	registry          RunningBackupsRegistry  // Stores the backup handler during execution.
	completionHandler BackupCompletionHandler // Runs post-success/failure actions (registry, retention, cluster config).
	backendService    BackupWriter            // Writes backup metadata and deletes created files on failure.
	startController   StartController         // Atomically reserves and attaches backup start.
	pathService       PathService             // Resolve backup path.
}

func NewBackupComponents(
	clientManager aerospike.ClientManager,
	backupExecutor backupexecutor.Backup,
	registry RunningBackupsRegistry,
	completionHandler BackupCompletionHandler,
	backendService BackupWriter,
	pathService PathService,
	startController StartController,
) *BackupComponents {
	return &BackupComponents{
		clientManager:     clientManager,
		backupExecutor:    backupExecutor,
		registry:          registry,
		completionHandler: completionHandler,
		backendService:    backendService,
		pathService:       pathService,
		startController:   startController,
	}
}

func newOrchestrator(
	routine *model.BackupRoutine,
	c *BackupComponents,
) *BackupRoutineOrchestrator {
	logger := slog.With(attr.Routine(routine.Name))
	retry := newRetryExecutor(*routine.BackupPolicy.GetRetryPolicyOrDefault(), logger, nonRetryableErrors...)
	runner := NewBackupNamespaceRunner(routine, c.backupExecutor, retry, c.backendService, logger, c.pathService)
	return &BackupRoutineOrchestrator{
		routine:           routine,
		runner:            runner,
		clientManager:     c.clientManager,
		registry:          c.registry,
		completionHandler: c.completionHandler,
		startController:   c.startController,
		logger:            logger,
	}
}

func (h *BackupRoutineOrchestrator) runBackup(ctx context.Context, now time.Time, backupType jobType) {
	release, err := h.startController.TryStart(h.routine, now, backupType)
	if err != nil {
		reportBackupOutcome(h.routine.Name, backupType, 0, err, h.logger)
		return
	}
	// Always release reservation to clear pending admission slot.
	defer release()

	duration, err := timeutil.MeasureDuration(func() error {
		return h.runBackupInternal(ctx, now, backupType)
	})

	reportBackupOutcome(h.routine.Name, backupType, duration, err, h.logger)
}

func (h *BackupRoutineOrchestrator) runBackupInternal(
	ctx context.Context,
	now time.Time,
	backupType jobType,
) error {
	h.logger.Info(backupType.String()+" backup started", slog.Time("now", now))

	client, namespaces, err := h.prepareCluster(ctx)
	if err != nil {
		return err
	}
	defer h.clientManager.Close(client)

	timeBounds := h.createTimeBounds(backupType, now)
	backupHandler := startNamespacesBackup(ctx, h.runner, client, namespaces, timeBounds, now, h.routine, backupType)
	h.registry.register(h.routine.Name, backupType, backupHandler)

	if err = backupHandler.Wait(ctx); err != nil {
		h.completionHandler.OnFailure(h.routine, backupType)
		return fmt.Errorf("%s backup failed: %w", backupType, err)
	}

	h.completionHandler.OnSuccess(ctx, h.routine, backupType, now, h.logger)

	return nil
}

func (h *BackupRoutineOrchestrator) prepareCluster(ctx context.Context) (aerospike.Client, []string, error) {
	client, err := h.clientManager.GetClient(ctx, h.routine.SourceCluster, h.logger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get backup client: %w", err)
	}

	namespaces := h.routine.Namespaces
	if len(namespaces) == 0 {
		namespaces, err = client.InfoClient().GetNamespacesList(ctx)
		if err != nil {
			h.clientManager.Close(client)
			return nil, nil, fmt.Errorf("failed to retrieve namespaces from source cluster: %w", err)
		}
	}

	return client, namespaces, nil
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
