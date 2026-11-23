package serverbackup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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

// routineRunner backs a routine up by asking the server to run one backup job per namespace, all
// at once. Runs of the routine may overlap under concurrent-incremental, each with its own jobs.
type routineRunner struct {
	clientManager     aerospike.ClientManager
	namespaceResolver aerospike.NamespaceResolver
	catalog           Catalog
}

var _ service.RoutineBackupRunner = (*routineRunner)(nil)

// NewRoutineRunner returns a RoutineBackupRunner that runs server-side backups. catalog lists the
// server backups an incremental backup of a namespace builds on.
func NewRoutineRunner(
	clientManager aerospike.ClientManager,
	namespaceResolver aerospike.NamespaceResolver,
	catalog Catalog,
) service.RoutineBackupRunner {
	return &routineRunner{
		clientManager:     clientManager,
		namespaceResolver: namespaceResolver,
		catalog:           catalog,
	}
}

// Run resolves the namespaces of the routine and starts backing them up in the background. The
// run is detached from ctx: only the handler's Cancel aborts its jobs. When ABS exits, the run
// ends with the process without aborting them, and they complete on the server.
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

	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	operation := newBackupOperation(cancel, len(namespaces))

	go func() {
		defer r.clientManager.Close(client)
		defer cancel()

		operation.finish(r.backupNamespaces(runCtx, client.InfoClient(), routine, runSpec, namespaces, operation, logger))
	}()

	return operation, nil
}

// backupNamespaces runs one server backup job per namespace, all at once, and joins their errors
// in the namespaces' order.
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

	var wg sync.WaitGroup
	errs := make([]error, len(namespaces))
	for i, namespace := range namespaces {
		wg.Go(func() {
			defer operation.finishNamespace(i)

			modifiedAfter := lowerBound(bounds, namespace)
			record := func(status *infomodels.ResponseBackupState) { operation.record(i, status) }
			err := r.backupNamespace(ctx, infoClient, routine, runSpec, modifiedAfter, namespace, record, logger)
			if err != nil {
				errs[i] = fmt.Errorf("namespace %s: %w", namespace, err)
			}
		})
	}
	wg.Wait()

	return errors.Join(errs...)
}

// backupNamespace runs the server backup job of namespace and passes each status of the job to
// record.
func (r *routineRunner) backupNamespace(
	ctx context.Context,
	infoClient backup.ServerBackupInfo,
	routine *model.BackupRoutine,
	runSpec model.BackupRunSpec,
	modifiedAfter *time.Time,
	namespace string,
	record func(*infomodels.ResponseBackupState),
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
	// As in scan mode, only a full backup stores the cluster configuration, and only when asked.
	withClusterConfig := runSpec.Type == model.BackupTypeFull && policy.WithClusterConfigOrDefault()
	jobID, err := infoClient.StartBackup(ctx, &infomodels.RequestBackup{
		RequestCommon:       common,
		NoConfig:            new(!withClusterConfig),
		BinList:             strings.Join(routine.BinList, ","),
		ModifiedAfter:       unixSeconds(modifiedAfter),
		ModifiedBefore:      unixSeconds(runSpec.TimeBounds.ToTime),
		BackupRPS:           ptr.ValueOrZero(policy.RecordsPerSecond),
		BackupWorkerThreads: ptr.ValueOrZero(policy.Parallel),
		BackupUploadThreads: ptr.ValueOrZero(policy.ParallelWrite),
	})

	job := &backupJob{infoClient: infoClient, jobID: jobID, record: record, logger: logger}
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
	// record receives each status of the job.
	record func(*infomodels.ResponseBackupState)
	logger *slog.Logger
}

// Wait waits for the job to end. If the job had no status for unobservedJobTimeout, or ctx is
// canceled, Wait aborts the job before it returns.
func (j *backupJob) Wait(ctx context.Context) error {
	status, err := j.poll(ctx)
	if err != nil {
		// The job may still be running on the server. The abort does not use ctx, because an
		// abort sent on a canceled ctx would fail and leave the job running.
		j.abort(context.WithoutCancel(ctx))
		return err
	}

	switch status.State {
	case infomodels.BackupStateFailed:
		return errJobEnded(backupJobKind, j.jobID, "failed", status.AbortOrigin, status.ErrorReason)
	case infomodels.BackupStateAborted:
		return errJobEnded(backupJobKind, j.jobID, "was aborted", status.AbortOrigin, status.ErrorReason)
	default:
		// The server completes a job that could not read some records, and the backup lacks them.
		if status.RecsReadFailed > 0 {
			j.logger.Warn("Server backup completed without the records the server could not read",
				slog.String("jobId", j.jobID),
				slog.String("namespace", status.Namespace),
				slog.Uint64("recordsReadFailed", status.RecsReadFailed))
		}

		return nil
	}
}

// poll polls the job until it ends and returns its final status. Each status is passed to record.
func (j *backupJob) poll(ctx context.Context) (*infomodels.ResponseBackupState, error) {
	fetch := func(ctx context.Context) (*infomodels.ResponseBackupState, error) {
		return j.infoClient.GetBackupStatus(ctx, j.jobID)
	}

	settled := func(status *infomodels.ResponseBackupState) (bool, error) {
		j.record(status)

		switch status.State {
		case infomodels.BackupStateComplete, infomodels.BackupStateFailed, infomodels.BackupStateAborted:
			return true, nil
		default:
			return false, nil
		}
	}

	return pollJob(ctx, j.logger, backupJobKind, j.jobID, fetch, settled)
}

// abort aborts the job on the server and waits up to cleanupTimeout for it to end, so that record
// receives the job's final status and abort origin.
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

	backupTypes := []model.BackupType{model.BackupTypeFull}
	if routine.BackupPolicy.GetIncrModeOrDefault() != model.IncrModeCumulative {
		backupTypes = append(backupTypes, model.BackupTypeIncremental)
	}

	bounds, err := r.catalog.LastBackupTimes(ctx, routine, backupTypes...)
	if err != nil {
		return nil, fmt.Errorf("failed to read the last backups of routine %s: %w", routine.Name, err)
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
	// namespaces holds the progress of the run's namespaces, one server job each, in the order the
	// run resolved them.
	namespaces []namespaceProgress
}

// namespaceProgress is the progress of the server job of one namespace.
type namespaceProgress struct {
	// job is the job as last reported, nil until it first reports.
	job *model.ServerBackupJob
	// progressPct is the job's progress in percent, as the server reports it.
	progressPct float64
	// metrics is the job's throughput, summed across the cluster's nodes.
	metrics models.Metrics
	// finished is set once the namespace's backup has ended, whatever its outcome. The job's state
	// cannot tell: a namespace whose start failed, or whose job went unobserved, has no final status.
	finished bool
}

var _ service.RoutineBackupHandler = (*backupOperation)(nil)

func newBackupOperation(cancel context.CancelFunc, namespaces int) *backupOperation {
	return &backupOperation{
		cancel:     cancel,
		startTime:  time.Now(),
		done:       make(chan struct{}),
		namespaces: make([]namespaceProgress, namespaces),
	}
}

// finish records the outcome of the run.
func (o *backupOperation) finish(err error) {
	o.err = err
	close(o.done)
}

// record stores the latest status of the job of namespace index i: the job, its progress and
// throughput.
func (o *backupOperation) record(i int, status *infomodels.ResponseBackupState) {
	job := newServerBackupJob(status)

	o.mu.Lock()
	defer o.mu.Unlock()

	ns := &o.namespaces[i]
	ns.job = &job
	ns.progressPct = status.ProgressPct
	ns.metrics = models.Metrics{
		RecordsPerSecond:   status.RecsPerSec,
		KilobytesPerSecond: status.BytesOutPerSec / 1024,
	}
}

// finishNamespace counts namespace index i as done, whatever the outcome of its backup.
func (o *backupOperation) finishNamespace(i int) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.namespaces[i].finished = true
}

// Status returns a snapshot of the run: its progress, throughput and server jobs. The server
// reports each namespace job's progress on its own, so every namespace weighs the same: a finished
// one counts whole and a running one by its job's progress. The throughput is the running jobs'
// sum. The server counts no records to back up, so Scan stays nil.
func (o *backupOperation) Status() *model.BackupRun {
	o.mu.Lock()
	defer o.mu.Unlock()

	var done float64
	var metrics models.Metrics
	jobs := make([]model.ServerBackupJob, 0, len(o.namespaces))
	for _, ns := range o.namespaces {
		if ns.job != nil {
			jobs = append(jobs, *ns.job)
		}

		if ns.finished {
			done++
			continue
		}

		done += ns.progressPct / 100
		metrics.RecordsPerSecond += ns.metrics.RecordsPerSecond
		metrics.KilobytesPerSecond += ns.metrics.KilobytesPerSecond
	}

	return &model.BackupRun{
		StartTime: o.startTime,
		Progress:  done / float64(len(o.namespaces)),
		Server:    &model.ServerProgress{Jobs: jobs, Metrics: &metrics},
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

// Cancel aborts the run's jobs that are still running.
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
