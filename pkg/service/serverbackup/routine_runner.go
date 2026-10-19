package serverbackup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/models"
	"github.com/aerospike/backup-go/pkg/asinfo"
	infomodels "github.com/aerospike/backup-go/pkg/asinfo/models"
)

const (
	// pollInterval is how often a running server job is asked for its status.
	pollInterval = time.Second
	// progressScale turns the server's progress percentage into the record counters
	// the service reports progress from.
	progressScale = 100
)

// routineRunner backs a routine up by asking the server to run one backup job per
// namespace. The jobs run one after another: the server runs a single job at a time.
type routineRunner struct {
	clientManager     aerospike.ClientManager
	namespaceResolver aerospike.NamespaceResolver
}

var _ service.RoutineBackupRunner = (*routineRunner)(nil)

// NewRoutineRunner returns a RoutineBackupRunner that runs server-side backups.
func NewRoutineRunner(
	clientManager aerospike.ClientManager,
	namespaceResolver aerospike.NamespaceResolver,
) service.RoutineBackupRunner {
	return &routineRunner{
		clientManager:     clientManager,
		namespaceResolver: namespaceResolver,
	}
}

// Run resolves the namespaces of the routine and starts backing them up in the background.
func (r *routineRunner) Run(
	ctx context.Context,
	routine *model.BackupRoutine,
	runSpec model.BackupRunSpec,
	logger *slog.Logger,
) (service.CancelableBackupHandler, error) {
	namespaces, err := r.namespaceResolver.ResolveNamespaces(ctx, routine, logger)
	if err != nil {
		return nil, err
	}

	client, err := r.clientManager.GetClient(ctx, routine.SourceCluster, nil, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to get backup client: %w", err)
	}

	runCtx, cancel := context.WithCancel(ctx)
	operation := newBackupOperation(cancel)

	go func() {
		defer r.clientManager.Close(client)
		defer cancel()

		operation.finish(r.backupNamespaces(runCtx, client.InfoClient(), routine, runSpec, namespaces, operation, logger))
	}()

	return operation, nil
}

// backupNamespaces runs one server backup job per namespace, in order, and joins their errors.
func (r *routineRunner) backupNamespaces(
	ctx context.Context,
	infoClient backup.ServerBackupInfo,
	routine *model.BackupRoutine,
	runSpec model.BackupRunSpec,
	namespaces []string,
	operation *backupOperation,
	logger *slog.Logger,
) error {
	var errs error
	for _, namespace := range namespaces {
		if err := r.backupNamespace(ctx, infoClient, routine, runSpec, namespace, operation, logger); err != nil {
			errs = errors.Join(errs, fmt.Errorf("namespace %s: %w", namespace, err))
		}
	}

	return errs
}

func (r *routineRunner) backupNamespace(
	ctx context.Context,
	infoClient backup.ServerBackupInfo,
	routine *model.BackupRoutine,
	runSpec model.BackupRunSpec,
	namespace string,
	operation *backupOperation,
	logger *slog.Logger,
) error {
	common, err := newRequestCommon(routine.Storage, namespace)
	if err != nil {
		return err
	}
	common.SetList = strings.Join(routine.SetList, ",")

	jobID, err := infoClient.StartBackup(ctx, &infomodels.RequestBackup{
		RequestCommon:  common,
		ModifiedAfter:  unixSeconds(runSpec.TimeBounds.FromTime),
		ModifiedBefore: unixSeconds(runSpec.TimeBounds.ToTime),
	})
	if err != nil {
		return fmt.Errorf("failed to start server backup: %w", err)
	}

	logger.Info("Server backup started", slog.String("namespace", namespace), slog.String("jobId", jobID))

	return waitForBackup(ctx, infoClient, jobID, operation, logger)
}

// waitForBackup polls the job until it ends. A canceled ctx aborts the job on the server.
func waitForBackup(
	ctx context.Context,
	infoClient backup.ServerBackupInfo,
	jobID string,
	operation *backupOperation,
	logger *slog.Logger,
) error {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if err := infoClient.AbortBackup(context.WithoutCancel(ctx), jobID); err != nil {
				logger.Warn("Failed to abort server backup", slog.String("jobId", jobID), attr.Error(err))
			}
			return ctx.Err()
		case <-ticker.C:
		}

		status, err := infoClient.GetBackupStatus(ctx, jobID)
		if errors.Is(err, asinfo.ErrNotFound) {
			continue // the job is not visible on the cluster yet.
		}
		if err != nil {
			return fmt.Errorf("failed to get server backup status: %w", err)
		}

		operation.reportProgress(status.ProgressPct)

		switch status.State {
		case infomodels.BackupStateComplete:
			return nil
		case infomodels.BackupStateFailed:
			return fmt.Errorf("server backup %s failed: %s", jobID, status.ErrorReason)
		case infomodels.BackupStateAborted:
			return fmt.Errorf("server backup %s was aborted", jobID)
		default:
		}
	}
}

func unixSeconds(t *time.Time) string {
	if t == nil {
		return ""
	}

	return strconv.FormatInt(t.Unix(), 10)
}

// backupOperation is the handle of a routine's server backup run. The run itself happens
// in the routine runner; the operation only reports its progress and outcome.
type backupOperation struct {
	cancel context.CancelFunc
	stats  *models.BackupStats
	done   chan struct{}
	err    error
}

var _ service.CancelableBackupHandler = (*backupOperation)(nil)

func newBackupOperation(cancel context.CancelFunc) *backupOperation {
	stats := models.NewBackupStats()
	stats.Start()
	stats.TotalRecords.Store(progressScale)

	return &backupOperation{
		cancel: cancel,
		stats:  stats,
		done:   make(chan struct{}),
	}
}

// reportProgress records the progress percentage of the namespace being backed up.
func (o *backupOperation) reportProgress(percent float64) {
	o.stats.ReadRecords.Store(uint64(percent))
}

func (o *backupOperation) finish(err error) {
	o.err = err
	o.stats.Stop()
	close(o.done)
}

// Wait waits for every namespace job of the run to end.
func (o *backupOperation) Wait(ctx context.Context) error {
	select {
	case <-o.done:
		return o.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Cancel aborts the running namespace job and skips the remaining ones.
func (o *backupOperation) Cancel() {
	o.cancel()
}

// GetStats returns the progress of the namespace being backed up.
func (o *backupOperation) GetStats() *models.BackupStats {
	return o.stats
}

// GetMetrics returns empty metrics: the server reports no client-side throughput.
func (o *backupOperation) GetMetrics() *models.Metrics {
	return &models.Metrics{}
}
