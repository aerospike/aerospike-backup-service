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
	"github.com/aerospike/backup-go/pkg/asinfo"
	infomodels "github.com/aerospike/backup-go/pkg/asinfo/models"
)

// routineRunner backs a routine up by asking the server to run one backup job per namespace.
// The jobs run one after another, because the server runs one backup job at a time.
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
	bounds, err := r.lowerBounds(ctx, routine, runSpec.Type)
	if err != nil {
		return err
	}

	var errs error
	for _, namespace := range namespaces {
		// StartBackup is sent even when ctx is already canceled, so stop here instead of starting
		// another job.
		if err := ctx.Err(); err != nil {
			return errors.Join(errs, err)
		}

		modifiedAfter := lowerBound(bounds, namespace)
		err := r.backupNamespace(ctx, infoClient, routine, runSpec, modifiedAfter, namespace, operation, logger)
		if err != nil {
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
	modifiedAfter *time.Time,
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
	}, routine.Storage, routine.Name)
	if err != nil {
		return err
	}

	// An unset limit is sent as zero. The info client leaves zero fields out of the command, so
	// the server uses its own default.
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

	job := &backupJob{infoClient: infoClient, jobID: jobID, operation: operation, logger: logger}
	if err != nil {
		if errors.Is(err, asinfo.ErrOutcomeUnknown) {
			// The server may have started the job even though the start failed.
			job.abort(context.WithoutCancel(ctx))
		}

		return fmt.Errorf("failed to start server backup: %w", err)
	}

	logger.Info("Server backup started", slog.String("namespace", namespace), slog.String("jobId", jobID))

	return job.Wait(ctx)
}

// backupJob follows one server backup job from its start to its end.
type backupJob struct {
	infoClient backup.ServerBackupInfo
	jobID      string
	// operation receives each status of the job.
	operation *backupOperation
	logger    *slog.Logger
}

// Wait waits for the job to end. If the wait fails, or ctx is canceled, Wait aborts the job
// before it returns.
func (j *backupJob) Wait(ctx context.Context) error {
	status, err := j.poll(ctx)
	if err != nil {
		// The job may still be running on the server. The abort does not use ctx, because a
		// canceled job would otherwise keep the cluster's backup slot.
		j.abort(context.WithoutCancel(ctx))
		return err
	}

	switch status.State {
	case infomodels.BackupStateFailed:
		return errJobEnded(backupJobKind, j.jobID, "failed", status.AbortOrigin, status.ErrorReason)
	case infomodels.BackupStateAborted:
		return errJobEnded(backupJobKind, j.jobID, "was aborted", status.AbortOrigin, status.ErrorReason)
	default:
		return nil
	}
}

// poll polls the job until it ends and returns its final status. Each status is recorded in the
// operation.
func (j *backupJob) poll(ctx context.Context) (*infomodels.ResponseBackupState, error) {
	fetch := func(ctx context.Context) (*infomodels.ResponseBackupState, error) {
		return j.infoClient.GetBackupStatus(ctx, j.jobID)
	}

	return pollJob(ctx, backupJobKind, j.jobID, fetch, func(status *infomodels.ResponseBackupState) (bool, error) {
		j.operation.record(status)

		switch status.State {
		case infomodels.BackupStateComplete, infomodels.BackupStateFailed, infomodels.BackupStateAborted:
			return true, nil
		default:
			return false, nil
		}
	})
}

// abort aborts the job on the server and waits up to cleanupTimeout for it to end, so that the
// operation holds the job's final status and abort origin.
func (j *backupJob) abort(ctx context.Context) {
	if err := j.infoClient.AbortBackup(ctx, j.jobID); err != nil {
		j.logger.Warn("Failed to abort server backup", slog.String("jobId", j.jobID), attr.Error(err))
		return
	}

	ctx, cancel := context.WithTimeout(ctx, cleanupTimeout)
	defer cancel()

	if _, err := j.poll(ctx); err != nil {
		j.logger.Warn("Aborted server backup did not end", slog.String("jobId", j.jobID), attr.Error(err))
	}
}

// lowerBounds returns, per namespace, the time an incremental backup of it starts from: the time
// of the namespace's last full backup in the cumulative mode, otherwise of its last backup of
// either type. A namespace with no backup has no entry. A full backup has no lower bound, so the
// map is nil. The storage is listed once for all namespaces of the run.
func (r *routineRunner) lowerBounds(
	ctx context.Context,
	routine *model.BackupRoutine,
	backupType model.BackupType,
) (map[string]time.Time, error) {
	if backupType == model.BackupTypeFull {
		return nil, nil
	}

	filters := []service.BackupFilter{service.NewFullBackupFilter(routine)}
	if routine.BackupPolicy.GetIncrModeOrDefault() != model.IncrModeCumulative {
		filters = append(filters, service.NewIncrementalBackupFilter(routine))
	}

	bounds := make(map[string]time.Time)
	for _, filter := range filters {
		backups, err := r.backupReader.GetBackups(ctx, filter)
		if err != nil {
			return nil, fmt.Errorf("failed to read the last backups of routine %s: %w", routine.Name, err)
		}

		for _, b := range backups {
			if last, ok := bounds[b.Namespace]; !ok || b.Created.After(last) {
				bounds[b.Namespace] = b.Created
			}
		}
	}

	return bounds, nil
}

// lowerBound returns the namespace's bound in bounds, or none if it has no backup to build on.
func lowerBound(bounds map[string]time.Time, namespace string) *time.Time {
	bound, ok := bounds[namespace]
	if !ok {
		return nil
	}

	return &bound
}

func unixSeconds(t *time.Time) string {
	if t == nil {
		return ""
	}

	return strconv.FormatInt(t.Unix(), 10)
}

// backupOperation is the handle of a routine's server backup run. The routine runner does the
// run. The operation reports the run's progress, outcome and server jobs.
type backupOperation struct {
	cancel    context.CancelFunc
	startTime time.Time
	done      chan struct{}
	err       error

	mu sync.Mutex
	// jobs are the server jobs of the run in the order they started, each as last reported.
	jobs []model.ServerBackupJob
	// progressPct is the progress of the running job in percent, as the server reports it.
	progressPct float64
}

var _ service.RoutineBackupHandler = (*backupOperation)(nil)

func newBackupOperation(cancel context.CancelFunc) *backupOperation {
	return &backupOperation{
		cancel:    cancel,
		startTime: time.Now(),
		done:      make(chan struct{}),
	}
}

// finish records the outcome of the run.
func (o *backupOperation) finish(err error) {
	o.err = err
	close(o.done)
}

// record stores the latest status of a job. The progress of that job is the progress of the run.
func (o *backupOperation) record(status *infomodels.ResponseBackupState) {
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

// Status returns a snapshot of the run: the progress the server reports for the job running now,
// and the server jobs of the run. The server counts no records to back up, so Scan stays nil.
func (o *backupOperation) Status() *model.BackupRun {
	o.mu.Lock()
	defer o.mu.Unlock()

	return &model.BackupRun{
		StartTime: o.startTime,
		Progress:  o.progressPct / 100,
		Server:    &model.ServerProgress{Jobs: slices.Clone(o.jobs)},
	}
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
