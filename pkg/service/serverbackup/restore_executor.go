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

	common, err := withStorage(infomodels.RequestCommon{
		Namespace: namespace,
		SetList:   strings.Join(request.Policy.SetList, ","),
		NoIndexes: request.Policy.NoIndexes,
		NoUDFs:    request.Policy.NoUdfs,
	}, request.SourceStorage)
	if err != nil {
		return nil, err
	}

	lane := laneOf(request.Policy.Mode)
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
		Policy:               lane.policy,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start server restore: %w", explainRefusal(err, namespace))
	}

	return newRestoreHandler(infoClient, namespace, jobID, lane), nil
}

// restoreLane is how the server runs a restore job of one mode: the policy it starts with, and how
// a job that does not complete is released.
type restoreLane struct {
	policy  infomodels.RestorePolicy
	release func(ctx context.Context, infoClient backup.ServerBackupInfo, namespace, jobID string) error
}

// laneOf returns the lane of mode. A fuzzy restore writes into the live namespace, so an aborted
// one holds nothing.
func laneOf(mode model.RestoreMode) restoreLane {
	if mode == model.RestoreModeFuzzy {
		return restoreLane{policy: infomodels.RestorePolicyFuzzy, release: abortRestore}
	}

	return restoreLane{policy: infomodels.RestorePolicyStrict, release: releaseNamespace}
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

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("server restore %s did not %s: %w", jobID, what, ctx.Err())
		case <-ticker.C:
		}

		status, err := infoClient.GetRestoreStatus(ctx, namespace, jobID)
		if errors.Is(err, asinfo.ErrNotFound) {
			continue // a node does not report the job yet.
		}
		if err != nil {
			return fmt.Errorf("failed to get server restore status: %w", err)
		}

		if done(status) {
			return nil
		}
	}
}

// abortRestore aborts the restore jobID.
func abortRestore(ctx context.Context, infoClient backup.ServerBackupInfo, namespace, jobID string) error {
	if err := infoClient.AbortRestore(ctx, namespace, jobID); err != nil {
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
// is canceled. A restore that does not complete is released as its lane releases it, even when
// ctx is canceled: a canceled restore holds the namespace as well.
func (h *restoreHandler) Wait(ctx context.Context) error {
	defer h.stats.Stop()

	if err := h.awaitComplete(ctx); err != nil {
		return errors.Join(err, h.lane.release(context.WithoutCancel(ctx), h.infoClient, h.namespace, h.jobID))
	}

	return nil
}

// awaitComplete polls the restore job until it completes and the namespace is open to clients
// again, clearing the storage of the nodes that hold data in the namespace of a cold restore in
// preparation. It fails when the job fails or is aborted, a truncate holds the namespace, or ctx is
// canceled.
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

		switch {
		case status.HeldReason == heldByTruncateState:
			return fmt.Errorf("server restore %s is held: %w", h.jobID, errTruncateState(h.namespace))
		case status.Clear == infomodels.ClearStateRequired:
			if err := h.infoClient.ClearRestoreStorage(ctx, h.namespace, h.jobID); err != nil {
				return fmt.Errorf("failed to clear namespace %s for server restore %s: %w", h.namespace, h.jobID, err)
			}
		// The job completes before the server releases the namespace to clients.
		case status.State == infomodels.RestoreStateComplete && status.NsState == infomodels.NamespaceStateNone:
			return nil
		case status.State == infomodels.RestoreStateFailed:
			return fmt.Errorf("server restore %s failed", h.jobID)
		case status.State == infomodels.RestoreStateAborted:
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
