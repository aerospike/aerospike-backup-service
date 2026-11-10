package serverbackup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/restoreexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/models"
	"github.com/aerospike/backup-go/pkg/asinfo"
	infomodels "github.com/aerospike/backup-go/pkg/asinfo/models"
)

// maxChainLength is the most backups the server restores in one job (AS_RESTORE_CHAIN_MAX_LINKS).
const maxChainLength = 32

// restoreExecutor asks the server to restore server backups, identified by their keys: a
// single backup, or a full backup and the incremental backups that follow it. The policy's mode
// picks how: a strict restore is cold, a fuzzy one writes into the live namespace. A
// restore that does not complete is aborted, and a cold one's namespace released to clients with
// whatever it wrote.
type restoreExecutor struct {
	backupReader service.BackupReader
}

var _ restoreexecutor.Restore = (*restoreExecutor)(nil)

// NewRestoreExecutor returns a Restore that runs server-side restores. backupReader tells
// which namespace a backup holds, to restore into when the request maps none.
func NewRestoreExecutor(backupReader service.BackupReader) restoreexecutor.Restore {
	return &restoreExecutor{backupReader: backupReader}
}

// Run starts, in the background, one server restore job that applies the backups of the requests
// in the order given, the full backup first; each request names one backup by its key in
// BackupDataPath. The requests share their storage, routine and policy, so the first one stands
// for all. Every failure, including one to start, comes from the handler's Wait.
func (r *restoreExecutor) Run(
	ctx context.Context,
	client aerospike.Client,
	logger *slog.Logger,
	requests ...*model.RestoreRequest,
) restoreexecutor.RestoreHandler {
	ctx, cancel := context.WithCancel(ctx)
	handler := newRestoreHandler(cancel)

	go func() {
		defer cancel()

		handler.finish(r.restore(ctx, client, logger, requests))
	}()

	return handler
}

// restore starts the server restore job of the requests and waits for it to end.
func (r *restoreExecutor) restore(
	ctx context.Context,
	client aerospike.Client,
	logger *slog.Logger,
	requests []*model.RestoreRequest,
) error {
	if len(requests) > maxChainLength {
		return fmt.Errorf("the server restores at most %d backups at once, the chain has %d",
			maxChainLength, len(requests))
	}

	routineName, backupIDs, err := jobIDsOf(requests)
	if err != nil {
		return err
	}

	job, err := r.start(ctx, client, requests[0], routineName, backupIDs)
	if err != nil {
		return err
	}

	logger.Info("Server restore started", slog.String("namespace", job.namespace), slog.String("jobId", job.jobID))

	return job.Wait(ctx)
}

// start starts a server restore job of backupIDs, oldest first, from the routine's folder, with
// the policy and storage of request, and returns the job to follow. request names the first
// backup.
func (r *restoreExecutor) start(
	ctx context.Context,
	client aerospike.Client,
	request *model.RestoreRequest,
	routineName string,
	backupIDs []string,
) (*restoreJob, error) {
	namespace, err := r.destinationNamespace(ctx, request)
	if err != nil {
		return nil, err
	}

	common, err := withStorage(infomodels.RequestCommon{
		Namespace: namespace,
		SetList:   strings.Join(request.Policy.SetList, ","),
		NoIndexes: request.Policy.NoIndexes,
		NoUDFs:    request.Policy.NoUdfs,
	}, request.SourceStorage, routineName)
	if err != nil {
		return nil, err
	}

	mode := modeOf(request.Policy.Mode)
	infoClient := client.InfoClient()

	// A limit the policy leaves unset is zero, which the request omits: the server's default applies.
	// A cold restore needs no other command to start: the server gates and prepares the namespace
	// on every node, and starts once every node is ready. A refused start gates nothing.
	jobID, err := infoClient.StartRestore(ctx, &infomodels.RequestRestore{
		RequestCommon:        common,
		BackupIDs:            strings.Join(backupIDs, ","),
		RestoreWorkerThreads: ptr.ValueOrZero(request.Policy.Parallel),
		RestoreRPS:           ptr.ValueOrZero(request.Policy.Tps),
		ExtraTTL:             int(ptr.ValueOrZero(request.Policy.ExtraTTL)),
		Policy:               mode.policy,
	})
	if err != nil {
		startErr := fmt.Errorf("failed to start server restore: %w", explainRefusal(err, namespace))
		if !errors.Is(err, asinfo.ErrOutcomeUnknown) {
			return nil, startErr
		}

		// The server may have started the job though the start failed.
		return nil, errors.Join(startErr, mode.release(context.WithoutCancel(ctx), infoClient, namespace, jobID))
	}

	return newRestoreJob(infoClient, namespace, jobID, mode), nil
}

// jobIDsOf returns the routine whose folder holds the requests' backups, and their job ids in the
// requests' order. A chain is backed up by one routine, so the first key names the routine.
func jobIDsOf(requests []*model.RestoreRequest) (string, []string, error) {
	routineName, _, err := parseBackupKey(requests[0].BackupDataPath)
	if err != nil {
		return "", nil, err
	}

	jobIDs := make([]string, 0, len(requests))
	for _, request := range requests {
		_, jobID, err := parseBackupKey(request.BackupDataPath)
		if err != nil {
			return "", nil, err
		}
		jobIDs = append(jobIDs, jobID)
	}

	return routineName, jobIDs, nil
}

// restoreMode is how the server runs a restore job of one restore mode: the policy it starts with,
// and how a job that does not complete is released.
type restoreMode struct {
	policy  infomodels.RestorePolicy
	release func(ctx context.Context, infoClient backup.ServerBackupInfo, namespace, jobID string) error
}

// modeOf returns how the server runs a restore of mode. A fuzzy restore writes into the live
// namespace, so an aborted one holds nothing.
func modeOf(mode model.RestoreMode) restoreMode {
	if mode == model.RestoreModeFuzzy {
		return restoreMode{policy: infomodels.RestorePolicyFuzzy, release: abortRestore}
	}

	return restoreMode{policy: infomodels.RestorePolicyStrict, release: releaseNamespace}
}

// truncateStateRefusal marks the server's refusal of a cold restore into a namespace that holds
// truncate state: the restored records keep their backup times, which the truncate filters out.
const truncateStateRefusal = "holds truncate state"

// heldByTruncateState is the held-reason of a node that holds the namespace of a cold restore in
// preparation because a truncate committed after the restore started.
const heldByTruncateState = "truncate-state-live"

// explainRefusal adds the remedy to a refusal whose own text points to the server log.
func explainRefusal(err error, namespace string) error {
	if !strings.Contains(err.Error(), truncateStateRefusal) {
		return err
	}

	return fmt.Errorf("%w: %w", errTruncateState(namespace), err)
}

// errTruncateState tells how to clear the truncate state that keeps a cold restore out of namespace.
func errTruncateState(namespace string) error {
	return fmt.Errorf("namespace %s holds truncate state, which would filter out the restored records: "+
		"clear it with the info command 'truncate-namespace-undo:namespace=%s', or "+
		"'truncate-undo:namespace=%s;set=<set>' for each truncated set the server log names, and retry",
		namespace, namespace, namespace)
}

// releaseNamespace aborts the cold restore jobID and opens namespace to clients with whatever the
// restore wrote: an aborted cold restore keeps the namespace from clients until restore-open. The
// abort takes effect in the background, and a node whose abort lands after restore-open closes the
// namespace again, so it opens the namespace only once the job has ended. The opening also takes
// effect in the background: it returns once the namespace is open.
func releaseNamespace(ctx context.Context, infoClient backup.ServerBackupInfo, namespace, jobID string) error {
	if err := abortRestore(ctx, infoClient, namespace, jobID); err != nil {
		return err
	}

	ended := (*infomodels.ResponseRestoreState).IsFinished
	if err := awaitRestoreStatus(ctx, infoClient, namespace, jobID, "end", ended); err != nil {
		return openFailure(namespace, jobID, err)
	}

	if err := infoClient.OpenRestore(ctx, namespace); err != nil {
		return openFailure(namespace, jobID, err)
	}

	open := func(status *infomodels.ResponseRestoreState) bool {
		return status.NsState == infomodels.NamespaceStateNone
	}
	if err := awaitRestoreStatus(ctx, infoClient, namespace, jobID, "open the namespace", open); err != nil {
		return openFailure(namespace, jobID, err)
	}

	return nil
}

func openFailure(namespace, jobID string, err error) error {
	return fmt.Errorf("failed to open namespace %s after server restore %s: %w", namespace, jobID, err)
}

// awaitRestoreStatus polls the restore jobID for up to abortTimeout, until its status is done.
// what names the step the job is waited on for.
func awaitRestoreStatus(
	ctx context.Context,
	infoClient backup.ServerBackupInfo,
	namespace, jobID, what string,
	done func(*infomodels.ResponseRestoreState) bool,
) error {
	ctx, cancel := context.WithTimeout(ctx, abortTimeout)
	defer cancel()

	err := pollRestore(ctx, infoClient, namespace, jobID,
		func(status *infomodels.ResponseRestoreState) (bool, error) { return done(status), nil })
	if err != nil {
		return fmt.Errorf("server restore %s did not %s: %w", jobID, what, err)
	}

	return nil
}

// restoreJobName names a server restore job in errors.
const restoreJobName = "server restore"

// pollRestore polls the restore jobID of namespace until settled reports that the wait is over.
func pollRestore(
	ctx context.Context,
	infoClient backup.ServerBackupInfo,
	namespace, jobID string,
	settled func(*infomodels.ResponseRestoreState) (bool, error),
) error {
	fetch := func(ctx context.Context) (*infomodels.ResponseRestoreState, error) {
		return infoClient.GetRestoreStatus(ctx, namespace, jobID)
	}
	_, err := pollJob(ctx, restoreJobName, jobID, fetch, settled)

	return err
}

// abortRestore aborts the restore jobID.
func abortRestore(ctx context.Context, infoClient backup.ServerBackupInfo, namespace, jobID string) error {
	if err := infoClient.AbortRestore(ctx, namespace, jobID); err != nil {
		return fmt.Errorf("failed to abort server restore %s: %w", jobID, err)
	}

	return nil
}

// destinationNamespace returns the namespace to restore into: the mapped destination, or
// else the namespace the request's backup was taken from.
func (r *restoreExecutor) destinationNamespace(ctx context.Context, request *model.RestoreRequest) (string, error) {
	backupKey := request.BackupDataPath
	if request.Policy.Namespace != nil && request.Policy.Namespace.Destination != "" {
		return request.Policy.Namespace.Destination, nil
	}

	backups, err := r.backupReader.GetBackups(ctx, service.NewPathFilter(backupKey, request.SourceStorage))
	if err != nil {
		return "", fmt.Errorf("failed to read server backup %s: %w", backupKey, err)
	}
	if len(backups) == 0 {
		return "", fmt.Errorf("server backup %s not found", backupKey)
	}

	return backups[0].Namespace, nil
}

// restoreHandler is the handle of a server restore run. The run itself happens in the restore
// executor; the handler reports its outcome and how long it has run.
type restoreHandler struct {
	// cancel stops the run.
	cancel context.CancelFunc
	stats  *models.RestoreStats
	done   chan struct{}
	err    error
}

var _ restoreexecutor.RestoreHandler = (*restoreHandler)(nil)

func newRestoreHandler(cancel context.CancelFunc) *restoreHandler {
	stats := models.NewRestoreStats()
	stats.Start()

	return &restoreHandler{cancel: cancel, stats: stats, done: make(chan struct{})}
}

// finish records the outcome of the run.
func (h *restoreHandler) finish(err error) {
	h.err = err
	h.stats.Stop()
	close(h.done)
}

// Wait waits for the run to end. If ctx ends first, Wait stops the run and returns once it has
// stopped: the job is aborted and, for a cold restore, its namespace opened to clients again.
func (h *restoreHandler) Wait(ctx context.Context) error {
	select {
	case <-h.done:
	case <-ctx.Done():
		h.cancel()
		<-h.done
	}

	return h.err
}

// GetStats returns the restore statistics; the server reports no record counts.
func (h *restoreHandler) GetStats() *models.RestoreStats {
	return h.stats
}

// GetMetrics returns empty metrics: the server reports no client-side throughput.
func (h *restoreHandler) GetMetrics() *models.Metrics {
	return &models.Metrics{}
}

// restoreJob follows a server restore job.
type restoreJob struct {
	infoClient backup.ServerBackupInfo
	namespace  string
	jobID      string
	mode       restoreMode
}

func newRestoreJob(infoClient backup.ServerBackupInfo, namespace, jobID string, mode restoreMode) *restoreJob {
	return &restoreJob{infoClient: infoClient, namespace: namespace, jobID: jobID, mode: mode}
}

// Wait polls the restore job until it ends and the namespace is open to clients again, or ctx
// is canceled. A restore that does not complete is released as its mode releases it, even when
// ctx is canceled: a canceled restore holds the namespace as well.
func (j *restoreJob) Wait(ctx context.Context) error {
	if err := j.awaitComplete(ctx); err != nil {
		return errors.Join(err, j.mode.release(context.WithoutCancel(ctx), j.infoClient, j.namespace, j.jobID))
	}

	return nil
}

// awaitComplete polls the restore job until it completes and the namespace is open to clients
// again. It fails when the job cannot complete, or ctx is canceled.
func (j *restoreJob) awaitComplete(ctx context.Context) error {
	return pollRestore(ctx, j.infoClient, j.namespace, j.jobID,
		func(status *infomodels.ResponseRestoreState) (bool, error) { return j.advance(ctx, status) })
}

// advance acts on a status of the job and reports whether the job has completed and the namespace
// is open to clients again. It clears the storage of the nodes that hold data in the namespace of
// a cold restore in preparation, and fails when the job fails or is aborted, or a truncate holds
// the namespace.
func (j *restoreJob) advance(ctx context.Context, status *infomodels.ResponseRestoreState) (bool, error) {
	switch {
	case status.HeldReason == heldByTruncateState:
		return false, fmt.Errorf("server restore %s is held: %w", j.jobID, errTruncateState(j.namespace))
	case status.Clear == infomodels.ClearStateRequired:
		if err := j.infoClient.ClearRestoreStorage(ctx, j.namespace, j.jobID); err != nil {
			return false, fmt.Errorf("failed to clear namespace %s for server restore %s: %w", j.namespace, j.jobID, err)
		}

		return false, nil
	case status.State == infomodels.RestoreStateFailed:
		return false, errJobEnded(restoreJobName, j.jobID, "failed", status.AbortOrigin, status.ErrorReason)
	case status.State == infomodels.RestoreStateAborted:
		return false, errJobEnded(restoreJobName, j.jobID, "was aborted", status.AbortOrigin, status.ErrorReason)
	default:
		// The job completes before the server releases the namespace to clients.
		return status.State == infomodels.RestoreStateComplete && status.NsState == infomodels.NamespaceStateNone, nil
	}
}
