package serverbackup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/pkg/asinfo"
	infomodels "github.com/aerospike/backup-go/pkg/asinfo/models"
)

// pollInterval is how often a running server job is asked for its status.
const pollInterval = time.Second

// routineRunner backs a routine up by asking the server to run one backup job per
// namespace. The jobs run one after another: the server runs a single job at a time.
type routineRunner struct {
	clientManager     aerospike.ClientManager
	namespaceResolver aerospike.NamespaceResolver
	backupReader      service.BackupReader
}

var _ service.RoutineBackupRunner = (*routineRunner)(nil)

// NewRoutineRunner returns a RoutineBackupRunner that runs server-side backups. backupReader
// lists the server backups an incremental backup of a namespace builds on.
func NewRoutineRunner(
	clientManager aerospike.ClientManager,
	namespaceResolver aerospike.NamespaceResolver,
	backupReader service.BackupReader,
) service.RoutineBackupRunner {
	return &routineRunner{
		clientManager:     clientManager,
		namespaceResolver: namespaceResolver,
		backupReader:      backupReader,
	}
}

// Run resolves the namespaces of the routine and starts backing them up in the background.
func (r *routineRunner) Run(
	ctx context.Context,
	routine *model.BackupRoutine,
	runSpec model.BackupRunSpec,
	logger *slog.Logger,
) (service.RoutineBackupHandler, error) {
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

		operation.finish(r.backupNamespaces(runCtx, client.InfoClient(), routine, runSpec, namespaces, logger))
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
	logger *slog.Logger,
) error {
	var errs error
	for _, namespace := range namespaces {
		if err := r.backupNamespace(ctx, infoClient, routine, runSpec, namespace, logger); err != nil {
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
	logger *slog.Logger,
) error {
	common, err := withStorage(infomodels.RequestCommon{Namespace: namespace}, routine.Storage)
	if err != nil {
		return err
	}

	modifiedAfter, err := r.modifiedAfter(ctx, routine, runSpec.Type, namespace)
	if err != nil {
		return err
	}

	jobID, err := infoClient.StartBackup(ctx, &infomodels.RequestBackup{
		RequestCommon:  common,
		ModifiedAfter:  unixSeconds(modifiedAfter),
		ModifiedBefore: unixSeconds(runSpec.TimeBounds.ToTime),
	})
	if err != nil {
		return fmt.Errorf("failed to start server backup: %w", err)
	}

	logger.Info("Server backup started", slog.String("namespace", namespace), slog.String("jobId", jobID))

	return waitForBackup(ctx, infoClient, jobID, logger)
}

// waitForBackup waits for the job to end. A wait that fails, a canceled ctx included, aborts the
// job on the server.
func waitForBackup(ctx context.Context, infoClient backup.ServerBackupInfo, jobID string, logger *slog.Logger) error {
	status, err := pollBackup(ctx, infoClient, jobID)
	if err != nil {
		// The job may still run on the server: abort it, even when ctx is canceled, so it does not
		// hold the cluster's backup slot after the run has given up on it.
		if abortErr := infoClient.AbortBackup(context.WithoutCancel(ctx), jobID); abortErr != nil {
			logger.Warn("Failed to abort server backup", slog.String("jobId", jobID), attr.Error(abortErr))
		}

		return err
	}

	switch status.State {
	case infomodels.BackupStateFailed:
		return fmt.Errorf("server backup %s failed: %s", jobID, status.ErrorReason)
	case infomodels.BackupStateAborted:
		return fmt.Errorf("server backup %s was aborted", jobID)
	default:
		return nil
	}
}

// pollBackup polls the job until it ends, and returns its final status.
func pollBackup(
	ctx context.Context,
	infoClient backup.ServerBackupInfo,
	jobID string,
) (*infomodels.ResponseBackupState, error) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}

		status, err := infoClient.GetBackupStatus(ctx, jobID)
		if errors.Is(err, asinfo.ErrNotFound) {
			continue // the job is not visible on the cluster yet.
		}
		if err != nil {
			return nil, fmt.Errorf("failed to get server backup status: %w", err)
		}

		switch status.State {
		case infomodels.BackupStateComplete, infomodels.BackupStateFailed, infomodels.BackupStateAborted:
			return status, nil
		default:
		}
	}
}

// modifiedAfter returns the lower bound of a backup of namespace: none for a full backup. An
// incremental backup starts where the namespace's own last backup it builds on started: its last
// full backup in the cumulative mode, else its last backup of either type. The namespaces of a
// routine are backed up one after another, so each has a bound of its own.
func (r *routineRunner) modifiedAfter(
	ctx context.Context,
	routine *model.BackupRoutine,
	backupType model.BackupType,
	namespace string,
) (*time.Time, error) {
	if backupType == model.BackupTypeFull {
		return nil, nil
	}

	filters := []service.BackupFilter{service.NewFullBackupFilter(routine)}
	if routine.BackupPolicy.GetIncrModeOrDefault() != model.IncrModeCumulative {
		filters = append(filters, service.NewIncrementalBackupFilter(routine))
	}

	var last *time.Time
	for _, filter := range filters {
		backups, err := r.backupReader.GetBackups(ctx, filter)
		if err != nil {
			return nil, fmt.Errorf("failed to read the last backup of namespace %s: %w", namespace, err)
		}

		for _, b := range backups {
			if b.Namespace == namespace && (last == nil || b.Created.After(*last)) {
				last = &b.Created
			}
		}
	}

	return last, nil
}

func unixSeconds(t *time.Time) string {
	if t == nil {
		return ""
	}

	return strconv.FormatInt(t.Unix(), 10)
}

// backupOperation is the handle of a routine's server backup run. The run itself happens
// in the routine runner; the operation only reports when it started and its outcome.
type backupOperation struct {
	cancel    context.CancelFunc
	startTime time.Time
	done      chan struct{}
	err       error
}

var _ service.RoutineBackupHandler = (*backupOperation)(nil)

func newBackupOperation(cancel context.CancelFunc) *backupOperation {
	return &backupOperation{
		cancel:    cancel,
		startTime: time.Now(),
		done:      make(chan struct{}),
	}
}

// Status is a snapshot of the run. The server counts no records to back up, so there is no
// scan slot.
func (o *backupOperation) Status() *model.BackupRun {
	return &model.BackupRun{StartTime: o.startTime}
}

func (o *backupOperation) finish(err error) {
	o.err = err
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
