package serverbackup

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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

// restoreExecutor asks the server to restore server backups, identified by their job ids: a
// single backup, or a full backup and the incremental backups that follow it. The policy's mode
// picks the lane: a strict restore is cold, a fuzzy one writes into the live namespace. A
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

// Run starts one server restore job that applies the full backup and then each incremental,
// in that order; each request names one backup by its job id in BackupDataPath. The requests
// share their policy and storage, so the full one stands for all.
func (r *restoreExecutor) Run(
	ctx context.Context,
	client aerospike.Client,
	full *model.RestoreRequest,
	incrementals ...*model.RestoreRequest,
) (restoreexecutor.RestoreHandler, error) {
	if len(incrementals)+1 > maxChainLength {
		return nil, fmt.Errorf("the server restores at most %d backups at once, the chain has %d",
			maxChainLength, len(incrementals)+1)
	}

	backupIDs := []string{full.BackupDataPath}
	for _, incremental := range incrementals {
		backupIDs = append(backupIDs, incremental.BackupDataPath)
	}

	return r.start(ctx, client, full, backupIDs)
}

// start starts a server restore job of backupIDs, oldest first, with the policy and storage of
// request.
func (r *restoreExecutor) start(
	ctx context.Context,
	client aerospike.Client,
	request *model.RestoreRequest,
	backupIDs []string,
) (restoreexecutor.RestoreHandler, error) {
	namespace, err := r.destinationNamespace(ctx, request, backupIDs[0])
	if err != nil {
		return nil, err
	}

	common, err := newRequestCommon(request.SourceStorage, namespace)
	if err != nil {
		return nil, err
	}
	common.SetList = strings.Join(request.Policy.SetList, ",")
	common.NoIndexes = request.Policy.NoIndexes
	common.NoUDFs = request.Policy.NoUdfs

	lane := laneOf(request.Policy.Mode)
	jobID := newJobID(time.Now())
	infoClient := client.InfoClient()
	if err := lane.prepare(ctx, infoClient, namespace, jobID); err != nil {
		return nil, err
	}

	// A limit the policy leaves unset is zero, which the request omits: the server's default applies.
	err = infoClient.StartRestore(ctx, &infomodels.RequestRestore{
		RequestCommon:        common,
		JobID:                jobID,
		BackupIDs:            strings.Join(backupIDs, ","),
		RestoreWorkerThreads: ptr.ValueOrZero(request.Policy.Parallel),
		RestoreRPS:           ptr.ValueOrZero(request.Policy.Tps),
		ExtraTTL:             int(ptr.ValueOrZero(request.Policy.ExtraTTL)),
		Policy:               lane.policy,
	})
	if err != nil {
		return nil, errors.Join(fmt.Errorf("failed to start server restore: %w", err),
			lane.release(ctx, infoClient, namespace, jobID))
	}

	return newRestoreHandler(infoClient, namespace, jobID, lane), nil
}

// restoreLane is how the server runs a restore job of one mode: the policy it starts with, what
// runs before it starts, and how a job that does not complete is released.
type restoreLane struct {
	policy  infomodels.RestorePolicy
	prepare func(ctx context.Context, infoClient backup.ServerBackupInfo, namespace, jobID string) error
	release func(ctx context.Context, infoClient backup.ServerBackupInfo, namespace, jobID string) error
}

// laneOf returns the lane of mode. A fuzzy restore writes into the live namespace, so it needs
// no preparation, and an aborted one holds nothing.
func laneOf(mode model.RestoreMode) restoreLane {
	if mode == model.RestoreModeFuzzy {
		return restoreLane{
			policy:  infomodels.RestorePolicyFuzzy,
			prepare: func(context.Context, backup.ServerBackupInfo, string, string) error { return nil },
			release: abortRestore,
		}
	}

	return restoreLane{
		policy:  infomodels.RestorePolicyStrict,
		prepare: prepareColdRestore,
		release: releaseNamespace,
	}
}

// prepareColdRestore takes namespace from clients for the cold restore jobID and waits until it
// is ready on every node. A refused preparation leaves the namespace open; one that fails later
// releases it.
func prepareColdRestore(ctx context.Context, infoClient backup.ServerBackupInfo, namespace, jobID string) error {
	err := infoClient.PrepareRestore(ctx, &infomodels.RequestPrepareRestore{Namespace: namespace, JobID: jobID})
	if err != nil {
		return fmt.Errorf("failed to prepare server restore: %w", explainPrepareRefusal(err, namespace))
	}

	if err := awaitPrepared(ctx, infoClient, namespace, jobID); err != nil {
		return errors.Join(err, releaseNamespace(ctx, infoClient, namespace, jobID))
	}

	return nil
}

// truncateStateRefusal marks the server's refusal of a cold restore into a namespace that holds
// truncate state: the restored records keep their backup times, which the truncate filters out.
const truncateStateRefusal = "holds truncate state"

// explainPrepareRefusal adds the remedy to a refusal whose own text points to the server log.
func explainPrepareRefusal(err error, namespace string) error {
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

// awaitPrepared waits until namespace is ready for the cold restore jobID on every node, clearing
// the storage of the nodes that still hold data in it.
func awaitPrepared(ctx context.Context, infoClient backup.ServerBackupInfo, namespace, jobID string) error {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}

		status, err := infoClient.GetRestoreStatus(ctx, namespace, jobID)
		if errors.Is(err, asinfo.ErrNotFound) {
			continue // the job is not visible on the cluster yet.
		}
		if err != nil {
			return fmt.Errorf("failed to get server restore status: %w", err)
		}

		switch {
		case status.NsState == infomodels.NamespaceStateReady:
			return nil
		case status.NsState == infomodels.NamespaceStateFailed:
			return fmt.Errorf("server restore %s failed to prepare namespace %s: %s", jobID, namespace, status.ErrorReason)
		case status.HeldReason == infomodels.HeldReasonTruncateStateLive:
			return fmt.Errorf("server restore %s is held: %w", jobID, errTruncateState(namespace))
		case status.Clear == infomodels.ClearStateRequired:
			if err := infoClient.ClearRestoreStorage(ctx, namespace, jobID); err != nil {
				return fmt.Errorf("failed to clear namespace %s for server restore %s: %w", namespace, jobID, err)
			}
		default:
		}
	}
}

// releaseNamespace aborts the cold restore jobID and opens namespace to clients with whatever the
// restore wrote: an aborted cold restore keeps the namespace from clients until restore-open. It
// runs even when ctx is canceled, since a canceled restore holds the namespace as well.
func releaseNamespace(ctx context.Context, infoClient backup.ServerBackupInfo, namespace, jobID string) error {
	ctx = context.WithoutCancel(ctx)

	if err := abortRestore(ctx, infoClient, namespace, jobID); err != nil {
		return err
	}

	if err := infoClient.OpenRestore(ctx, namespace, jobID); err != nil {
		return fmt.Errorf("failed to open namespace %s after server restore %s: %w", namespace, jobID, err)
	}

	return nil
}

// abortRestore aborts the restore jobID, even when ctx is canceled.
func abortRestore(ctx context.Context, infoClient backup.ServerBackupInfo, namespace, jobID string) error {
	if err := infoClient.AbortRestore(context.WithoutCancel(ctx), namespace, jobID); err != nil {
		return fmt.Errorf("failed to abort server restore %s: %w", jobID, err)
	}

	return nil
}

// destinationNamespace returns the namespace to restore into: the mapped destination, or
// else the namespace backupID was taken from.
func (r *restoreExecutor) destinationNamespace(
	ctx context.Context,
	request *model.RestoreRequest,
	backupID string,
) (string, error) {
	if request.Policy.Namespace != nil && request.Policy.Namespace.Destination != "" {
		return request.Policy.Namespace.Destination, nil
	}

	backups, err := r.backupReader.GetBackups(ctx, service.NewPathFilter(backupID, request.SourceStorage))
	if err != nil {
		return "", fmt.Errorf("failed to read server backup %s: %w", backupID, err)
	}
	if len(backups) == 0 {
		return "", fmt.Errorf("server backup %s not found", backupID)
	}

	return backups[0].Namespace, nil
}

// restoreHandler follows a server restore job.
type restoreHandler struct {
	infoClient backup.ServerBackupInfo
	namespace  string
	jobID      string
	lane       restoreLane
	stats      *models.RestoreStats
}

var _ restoreexecutor.RestoreHandler = (*restoreHandler)(nil)

func newRestoreHandler(infoClient backup.ServerBackupInfo, namespace, jobID string, lane restoreLane) *restoreHandler {
	stats := models.NewRestoreStats()
	stats.Start()

	return &restoreHandler{
		infoClient: infoClient,
		namespace:  namespace,
		jobID:      jobID,
		lane:       lane,
		stats:      stats,
	}
}

// Wait polls the restore job until it ends and the namespace is open to clients again, or ctx
// is canceled. A restore that does not complete is released as its lane releases it.
func (h *restoreHandler) Wait(ctx context.Context) error {
	defer h.stats.Stop()

	if err := h.awaitComplete(ctx); err != nil {
		return errors.Join(err, h.lane.release(ctx, h.infoClient, h.namespace, h.jobID))
	}

	return nil
}

// awaitComplete polls the restore job until it completes and the namespace is open to clients
// again. It fails when the job fails or is aborted, or ctx is canceled.
func (h *restoreHandler) awaitComplete(ctx context.Context) error {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}

		status, err := h.infoClient.GetRestoreStatus(ctx, h.namespace, h.jobID)
		if errors.Is(err, asinfo.ErrNotFound) {
			continue // the job is not visible on the cluster yet.
		}
		if err != nil {
			return fmt.Errorf("failed to get server restore status: %w", err)
		}

		switch status.State {
		case infomodels.RestoreStateComplete:
			// The job completes before the server releases the namespace to clients.
			if status.NsState == infomodels.NamespaceStateNone {
				return nil
			}
		case infomodels.RestoreStateFailed:
			return fmt.Errorf("server restore %s failed: %s", h.jobID, status.ErrorReason)
		case infomodels.RestoreStateAborted:
			return fmt.Errorf("server restore %s was aborted", h.jobID)
		default:
		}
	}
}

// GetStats returns the restore statistics; the server reports no record counts.
func (h *restoreHandler) GetStats() *models.RestoreStats {
	return h.stats
}

// GetMetrics returns empty metrics: the server reports no client-side throughput.
func (h *restoreHandler) GetMetrics() *models.Metrics {
	return &models.Metrics{}
}
