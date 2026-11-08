package serverbackup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/models"
	"github.com/aerospike/backup-go/pkg/asinfo"
	infomodels "github.com/aerospike/backup-go/pkg/asinfo/models"
)

const (
	// pollInterval is how often a running server job is asked for its status.
	pollInterval = time.Second
	// abortTimeout bounds the wait for an aborted server job to end.
	abortTimeout = 30 * time.Second
)

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
	common, err := withStorage(infomodels.RequestCommon{
		Namespace: namespace,
		SetList:   strings.Join(routine.SetList, ","),
		FilterExp: routine.FilterExpression,
		NoIndexes: routine.BackupPolicy.NoIndexes,
		NoUDFs:    routine.BackupPolicy.NoUdfs,
	}, routine.Storage)
	if err != nil {
		return err
	}

	modifiedAfter, err := r.modifiedAfter(ctx, routine, runSpec.Type, namespace)
	if err != nil {
		return err
	}

	// A limit the policy leaves unset is zero, which the request omits: the server's default applies.
	policy := routine.BackupPolicy
	jobID, err := infoClient.StartBackup(ctx, &infomodels.RequestBackup{
		RequestCommon:       common,
		BinList:             strings.Join(routine.BinList, ","),
		ModifiedAfter:       unixSeconds(modifiedAfter),
		ModifiedBefore:      unixSeconds(runSpec.TimeBounds.ToTime),
		BackupRPS:           ptr.ValueOrZero(policy.RecordsPerSecond),
		BackupWorkerThreads: ptr.ValueOrZero(policy.Parallel),
		BackupUploadThreads: ptr.ValueOrZero(policy.ParallelWrite),
	})
	if err != nil {
		return fmt.Errorf("failed to start server backup: %w", err)
	}

	logger.Info("Server backup started", slog.String("namespace", namespace), slog.String("jobId", jobID))

	return waitForBackup(ctx, infoClient, jobID, operation, logger)
}

// waitForBackup waits for the job to end. A canceled ctx aborts the job on the server.
func waitForBackup(
	ctx context.Context,
	infoClient backup.ServerBackupInfo,
	jobID string,
	operation *backupOperation,
	logger *slog.Logger,
) error {
	status, err := pollBackup(ctx, infoClient, jobID, operation)
	if ctx.Err() != nil {
		abortJob(context.WithoutCancel(ctx), infoClient, jobID, operation, logger)
		return ctx.Err()
	}
	if err != nil {
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

// pollBackup polls the job, recording each status in operation, until the job ends, and
// returns its final status.
func pollBackup(
	ctx context.Context,
	infoClient backup.ServerBackupInfo,
	jobID string,
	operation *backupOperation,
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

		operation.report(status)

		switch status.State {
		case infomodels.BackupStateComplete, infomodels.BackupStateFailed, infomodels.BackupStateAborted:
			return status, nil
		default:
		}
	}
}

// abortJob aborts the job on the server and waits, up to abortTimeout, for the abort to end it,
// so the job is recorded as the server ends it.
func abortJob(
	ctx context.Context,
	infoClient backup.ServerBackupInfo,
	jobID string,
	operation *backupOperation,
	logger *slog.Logger,
) {
	if err := infoClient.AbortBackup(ctx, jobID); err != nil {
		logger.Warn("Failed to abort server backup", slog.String("jobId", jobID), attr.Error(err))
		return
	}

	ctx, cancel := context.WithTimeout(ctx, abortTimeout)
	defer cancel()

	if _, err := pollBackup(ctx, infoClient, jobID, operation); err != nil {
		logger.Warn("Aborted server backup did not end", slog.String("jobId", jobID), attr.Error(err))
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
// in the routine runner; the operation only reports its progress, outcome and server jobs.
type backupOperation struct {
	cancel context.CancelFunc
	stats  *models.BackupStats
	done   chan struct{}
	err    error

	mu sync.Mutex
	// jobs are the server jobs of the run in the order they started, each as last reported.
	jobs []model.ServerBackupJob
	// progressPct is the progress of the running job in percent, as the server reports it.
	progressPct float64
}

var (
	_ service.CancelableBackupHandler = (*backupOperation)(nil)
	_ service.ServerJobReporter       = (*backupOperation)(nil)
	_ service.ProgressReporter        = (*backupOperation)(nil)
)

func newBackupOperation(cancel context.CancelFunc) *backupOperation {
	stats := models.NewBackupStats()
	stats.Start()

	return &backupOperation{
		cancel: cancel,
		stats:  stats,
		done:   make(chan struct{}),
	}
}

// report records the latest status of a job, whose progress is the progress of the run.
func (o *backupOperation) report(status *infomodels.ResponseBackupState) {
	job := newServerBackupJob(status)

	o.mu.Lock()
	defer o.mu.Unlock()

	o.progressPct = status.ProgressPct

	if last := len(o.jobs) - 1; last >= 0 && o.jobs[last].JobID == job.JobID {
		o.jobs[last] = job
		return
	}
	o.jobs = append(o.jobs, job)
}

// ServerJobs returns the server jobs of the run, each as last reported.
func (o *backupOperation) ServerJobs() []model.ServerBackupJob {
	o.mu.Lock()
	defer o.mu.Unlock()

	return slices.Clone(o.jobs)
}

// Progress returns the fraction of the running job done. The server counts no records to
// back up, so the run has no record counts to report progress from.
func (o *backupOperation) Progress() float64 {
	o.mu.Lock()
	defer o.mu.Unlock()

	return o.progressPct / 100
}

func newServerBackupJob(status *infomodels.ResponseBackupState) model.ServerBackupJob {
	return model.ServerBackupJob{
		JobID:             status.JobID,
		Namespace:         status.Namespace,
		State:             string(status.State),
		ErrorReason:       status.ErrorReason,
		AbortOrigin:       string(status.AbortOrigin),
		StartTime:         nonZeroTime(status.StartTime),
		FinishTime:        nonZeroTime(status.FinishTime),
		RecordsBackedUp:   status.RecsBackedUp,
		BytesBackedUp:     status.BytesBackedUp,
		RecordsReadFailed: status.RecsReadFailed,
	}
}

func nonZeroTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}

	return &t
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

// GetStats returns when the run started; the server reports no record counts.
func (o *backupOperation) GetStats() *models.BackupStats {
	return o.stats
}

// GetMetrics returns empty metrics: the server reports no client-side throughput.
func (o *backupOperation) GetMetrics() *models.Metrics {
	return &models.Metrics{}
}
