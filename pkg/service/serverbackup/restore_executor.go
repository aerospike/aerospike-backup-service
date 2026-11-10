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
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/restoreexecutor"
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

var _ restoreexecutor.Restore = (*restoreExecutor)(nil)

// NewRestoreExecutor returns a Restore that runs server-side restores. backupReader tells
// which namespace a backup holds, to restore into.
func NewRestoreExecutor(backupReader service.BackupReader) restoreexecutor.Restore {
	return &restoreExecutor{backupReader: backupReader}
}

// Run starts, in the background, one server restore job that applies the backups of the requests
// in the order given, the full backup first; each request names one backup by its job id in
// BackupDataPath. The requests share their storage, so the first one stands for all.
// Every failure, including one to start, comes from the handler's Wait.
func (r *restoreExecutor) Run(
	ctx context.Context,
	client aerospike.Client,
	logger *slog.Logger,
	requests ...*model.RestoreRequest,
) restoreexecutor.RestoreHandler {
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
	backupIDs := make([]string, 0, len(requests))
	for _, request := range requests {
		backupIDs = append(backupIDs, request.BackupDataPath)
	}

	job, err := r.start(ctx, client, requests[0].SourceStorage, backupIDs)
	if err != nil {
		return err
	}

	logger.Info("Server restore started", slog.String("namespace", job.namespace), slog.String("jobId", job.jobID))

	return job.Wait(ctx)
}

// start starts a server restore job of backupIDs, oldest first, from storage, and returns the job
// to follow.
func (r *restoreExecutor) start(
	ctx context.Context,
	client aerospike.Client,
	storage model.Storage,
	backupIDs []string,
) (*restoreJob, error) {
	namespace, err := r.namespaceOf(ctx, storage, backupIDs[0])
	if err != nil {
		return nil, err
	}

	common, err := withStorage(infomodels.RequestCommon{Namespace: namespace}, storage)
	if err != nil {
		return nil, err
	}

	infoClient := client.InfoClient()
	jobID, err := infoClient.StartRestore(ctx, &infomodels.RequestRestore{
		RequestCommon: common,
		BackupIDs:     strings.Join(backupIDs, ","),
		Policy:        infomodels.RestorePolicyFuzzy,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start server restore: %w", err)
	}

	return &restoreJob{infoClient: infoClient, namespace: namespace, jobID: jobID}, nil
}

// namespaceOf returns the namespace backupID was taken from, which the restore writes into.
func (r *restoreExecutor) namespaceOf(ctx context.Context, storage model.Storage, backupID string) (string, error) {
	backups, err := r.backupReader.GetBackups(ctx, service.NewPathFilter(backupID, storage))
	if err != nil {
		return "", fmt.Errorf("failed to read server backup %s: %w", backupID, err)
	}
	if len(backups) == 0 {
		return "", fmt.Errorf("server backup %s not found", backupID)
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

	if abortErr := h.infoClient.AbortRestore(context.WithoutCancel(ctx), h.namespace, h.jobID); abortErr != nil {
		return errors.Join(err, fmt.Errorf("failed to abort server restore %s: %w", h.jobID, abortErr))
	}

	return err
}

// awaitComplete polls the restore job until it completes. It fails when the job fails or is
// aborted, or ctx is canceled.
func (h *restoreJob) awaitComplete(ctx context.Context) error {
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
			return nil
		case infomodels.RestoreStateFailed:
			return fmt.Errorf("server restore %s failed", h.jobID)
		case infomodels.RestoreStateAborted:
			return fmt.Errorf("server restore %s was aborted", h.jobID)
		default:
		}
	}
}
