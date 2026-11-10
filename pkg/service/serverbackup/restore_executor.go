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
	"github.com/aerospike/backup-go/models"
	"github.com/aerospike/backup-go/pkg/asinfo"
	infomodels "github.com/aerospike/backup-go/pkg/asinfo/models"
)

// maxChainLength is the most backups the server restores in one job (AS_RESTORE_CHAIN_MAX_LINKS).
const maxChainLength = 32

// restoreExecutor asks the server to restore server backups: a single backup, or a full backup
// and the incremental backups that follow it. The restore policy's mode selects a strict or a
// fuzzy restore. restoreJob drives the job once it has started.
type restoreExecutor struct {
	backupReader service.BackupReader
}

var _ restoreexecutor.Restore = (*restoreExecutor)(nil)

// NewRestoreExecutor returns a Restore that runs server-side restores. It reads a backup's
// namespace from backupReader when the request does not name a destination namespace.
func NewRestoreExecutor(backupReader service.BackupReader) restoreexecutor.Restore {
	return &restoreExecutor{backupReader: backupReader}
}

// Run starts one server restore job in the background. The job applies the backups of the
// requests in the order given, the full backup first. Each request names one backup by its key in
// BackupDataPath. All requests have the same storage, routine and policy. Every failure,
// including a failure to start, is returned by the handler's Wait.
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

		handler.finish(r.restore(ctx, client, logger, handler.stats, requests))
	}()

	return handler
}

// restore starts the server restore job of the requests and waits for it to end. The job's
// record counts are stored in stats.
func (r *restoreExecutor) restore(
	ctx context.Context,
	client aerospike.Client,
	logger *slog.Logger,
	stats *models.RestoreStats,
	requests []*model.RestoreRequest,
) error {
	if len(requests) > maxChainLength {
		return fmt.Errorf("restore chain has %d backups, the server restores at most %d in one job",
			len(requests), maxChainLength)
	}

	job, err := r.start(ctx, client, requests, stats)
	if err != nil {
		return err
	}

	logger.Info("Server restore started", slog.String("namespace", job.namespace), slog.String("jobId", job.jobID))

	return job.Wait(ctx)
}

// start sends the restore command for the backups of the requests and returns the job to follow.
func (r *restoreExecutor) start(
	ctx context.Context,
	client aerospike.Client,
	requests []*model.RestoreRequest,
	stats *models.RestoreStats,
) (*restoreJob, error) {
	routineName, backupIDs, err := backupIDsOf(requests)
	if err != nil {
		return nil, err
	}

	// The requests differ only in the backup they name.
	request := requests[0]

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

	// An unset limit is sent as zero. The info client leaves zero fields out of the command, so
	// the server uses its own default. The retry settings are set only for a fuzzy restore,
	// because the DTO layer rejects a retry policy on a strict one.
	retry := request.Policy.RetryPolicy
	infoClient := client.InfoClient()
	jobID, err := infoClient.StartRestore(ctx, &infomodels.RequestRestore{
		RequestCommon:        common,
		BackupIDs:            strings.Join(backupIDs, ","),
		RestoreWorkerThreads: ptr.ValueOrZero(request.Policy.Parallel),
		RestoreRPS:           ptr.ValueOrZero(request.Policy.Tps),
		ExtraTTL:             int(ptr.ValueOrZero(request.Policy.ExtraTTL)),
		// The server's restore policies have the names of the restore modes.
		Policy:              infomodels.RestorePolicy(request.Policy.Mode),
		RetryBaseIntervalMs: retry.BaseTimeoutMsOrNil(),
		RetryMultiplier:     retry.MultiplierOrZero(),
		RetryMaxAttempts:    retry.MaxRetriesOrNil(),
	})

	job := newRestoreJob(infoClient, namespace, jobID, request.Policy.Mode, stats)
	if err != nil {
		startErr := fmt.Errorf("failed to start server restore: %w", explainRefusal(err, namespace))
		if !errors.Is(err, asinfo.ErrOutcomeUnknown) {
			return nil, startErr
		}

		// The server may have started the job even though the start failed.
		return nil, errors.Join(startErr, job.release(context.WithoutCancel(ctx)))
	}

	return job, nil
}

// backupIDsOf returns the routine whose folder holds the requests' backups, and the ids of those
// backups in the requests' order. A backup's id is the id of the server job that wrote it. One
// routine writes all backups of a chain, so the first key names the routine.
func backupIDsOf(requests []*model.RestoreRequest) (string, []string, error) {
	routineName, _, err := parseBackupKey(requests[0].BackupDataPath)
	if err != nil {
		return "", nil, err
	}

	backupIDs := make([]string, 0, len(requests))
	for _, request := range requests {
		_, backupID, err := parseBackupKey(request.BackupDataPath)
		if err != nil {
			return "", nil, err
		}
		backupIDs = append(backupIDs, backupID)
	}

	return routineName, backupIDs, nil
}

// destinationNamespace returns the namespace to restore into: the destination the policy names,
// or else the namespace the request's backup was taken from.
func (r *restoreExecutor) destinationNamespace(ctx context.Context, request *model.RestoreRequest) (string, error) {
	if request.Policy.Namespace != nil && request.Policy.Namespace.Destination != "" {
		return request.Policy.Namespace.Destination, nil
	}

	backupKey := request.BackupDataPath
	backups, err := r.backupReader.GetBackups(ctx, service.NewPathFilter(backupKey, request.SourceStorage))
	if err != nil {
		return "", fmt.Errorf("failed to read server backup %s: %w", backupKey, err)
	}
	if len(backups) == 0 {
		return "", fmt.Errorf("server backup %s not found", backupKey)
	}

	return backups[0].Namespace, nil
}

// restoreHandler is the handle of a server restore run. The restore executor does the run. The
// handler reports the run's outcome, duration and record counts.
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

// Wait waits for the run to end. If ctx ends first, Wait stops the run and returns after the job
// has been released.
func (h *restoreHandler) Wait(ctx context.Context) error {
	select {
	case <-h.done:
	case <-ctx.Done():
		h.cancel()
		<-h.done
	}

	return h.err
}

// GetStats returns the duration of the run and the record counts the server reports for the job.
func (h *restoreHandler) GetStats() *models.RestoreStats {
	return h.stats
}

// GetMetrics returns empty metrics, because the server reports no client-side throughput.
func (h *restoreHandler) GetMetrics() *models.Metrics {
	return &models.Metrics{}
}
