package serverbackup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/models"
	"github.com/aerospike/backup-go/pkg/asinfo"
	infomodels "github.com/aerospike/backup-go/pkg/asinfo/models"
)

// restoreExecutor asks the server to restore server backups, identified by their job ids: a
// single backup, or a full backup and the incremental backups that follow it. The server writes
// the backed-up records into the live namespace (the fuzzy restore policy), which clients keep
// using. A restore that does not complete is aborted.
type restoreExecutor struct {
	backupReader service.BackupReader
}

var _ service.Restore = (*restoreExecutor)(nil)

// NewRestoreExecutor returns a Restore that runs server-side restores. backupReader tells
// which namespace a backup holds, to restore into.
func NewRestoreExecutor(backupReader service.BackupReader) service.Restore {
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
) service.RestoreHandler {
	ctx, cancel := context.WithCancel(ctx)
	handler := newRestoreHandler(cancel)

	go func() {
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
	job, err := r.start(ctx, client, requests)
	if err != nil {
		return err
	}

	logger.Info("Server restore started", slog.String("namespace", job.namespace), slog.String("jobId", job.jobID))

	return job.Wait(ctx)
}

// start starts a server restore job of the requests' backups, oldest first, and returns the job
// to follow.
func (r *restoreExecutor) start(
	ctx context.Context,
	client aerospike.Client,
	requests []*model.RestoreRequest,
) (*restoreJob, error) {
	first := requests[0]
	routineName, backupIDs, err := jobIDsOf(requests)
	if err != nil {
		return nil, err
	}

	namespace, err := r.namespaceOf(ctx, first.SourceStorage, first.BackupDataPath)
	if err != nil {
		return nil, err
	}

	// The server writes into the namespace the backup was taken from: refuse a remap rather than
	// write into a namespace the request left out.
	if destination := first.Policy.Namespace.DestinationOr(namespace); destination != namespace {
		return nil, fmt.Errorf("server restore cannot restore namespace %s into %s", namespace, destination)
	}

	common, err := withStorage(infomodels.RequestCommon{Namespace: namespace}, first.SourceStorage, routineName)
	if err != nil {
		return nil, err
	}

	infoClient := client.InfoClient()
	jobID, err := infoClient.StartRestore(ctx, &infomodels.RequestRestore{
		RequestCommon: common,
		BackupIDs:     strings.Join(backupIDs, ","),
		Policy:        infomodels.RestorePolicyFuzzy,
	})
	job := &restoreJob{infoClient: infoClient, namespace: namespace, jobID: jobID}
	if err != nil {
		startErr := fmt.Errorf("failed to start server restore: %w", err)
		if !errors.Is(err, asinfo.ErrOutcomeUnknown) {
			return nil, startErr
		}

		// The server may have started the job though the start failed.
		return nil, errors.Join(startErr, job.abort(ctx))
	}

	return job, nil
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

// namespaceOf returns the namespace the backup at key was taken from.
func (r *restoreExecutor) namespaceOf(ctx context.Context, storage model.Storage, key string) (string, error) {
	backups, err := r.backupReader.GetBackups(ctx, service.NewPathFilter(key, storage))
	if err != nil {
		return "", fmt.Errorf("failed to read server backup %s: %w", key, err)
	}
	if len(backups) == 0 {
		return "", fmt.Errorf("server backup %s not found", key)
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

var _ service.RestoreHandler = (*restoreHandler)(nil)

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
// stopped, with the server job aborted.
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
}

// Wait polls the restore job until it ends, or ctx is canceled. A restore that does not complete
// is aborted, even when ctx is canceled, so that it stops writing into the namespace.
func (h *restoreJob) Wait(ctx context.Context) error {
	err := h.awaitComplete(ctx)
	if err == nil {
		return nil
	}

	return errors.Join(err, h.abort(ctx))
}

// abort aborts the job, even when ctx is canceled, so that it stops writing into the namespace.
// A failed abort is returned: the job may still be writing.
func (h *restoreJob) abort(ctx context.Context) error {
	if err := h.infoClient.AbortRestore(context.WithoutCancel(ctx), h.namespace, h.jobID); err != nil {
		return fmt.Errorf("failed to abort server restore %s: %w", h.jobID, err)
	}

	return nil
}

// awaitComplete polls the restore job until it completes. It fails when the job fails or is
// aborted, or ctx is canceled.
func (h *restoreJob) awaitComplete(ctx context.Context) error {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	seen := false
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}

		status, err := h.infoClient.GetRestoreStatus(ctx, h.namespace, h.jobID)
		if errors.Is(err, asinfo.ErrNotFound) && !seen {
			// The job is not visible on every node yet. Once it has been, a node that does not
			// know it (one that joined or restarted) means the job can no longer be followed.
			continue
		}
		if err != nil {
			return fmt.Errorf("failed to get server restore status: %w", err)
		}
		seen = true

		switch status.State {
		case infomodels.RestoreStateComplete:
			return nil
		case infomodels.RestoreStateFailed:
			return fmt.Errorf("server restore %s failed", h.jobID)
		case infomodels.RestoreStateAborted:
			return fmt.Errorf("server restore %s was aborted", h.jobID)
		default:
		}
	}
}
