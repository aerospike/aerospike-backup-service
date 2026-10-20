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

// restoreExecutor asks the server to run a fuzzy restore of a server backup, identified by the
// job id of the backup.
type restoreExecutor struct {
	backupReader service.BackupReader
}

var _ restoreexecutor.Restore = (*restoreExecutor)(nil)

// NewRestoreExecutor returns a Restore that runs server-side restores. backupReader tells
// which namespace a backup holds, to restore into when the request maps none.
func NewRestoreExecutor(backupReader service.BackupReader) restoreexecutor.Restore {
	return &restoreExecutor{backupReader: backupReader}
}

// Run starts the server restore of the job id in request.BackupDataPath.
func (r *restoreExecutor) Run(
	ctx context.Context,
	client aerospike.Client,
	request *model.RestoreRequest,
) (restoreexecutor.RestoreHandler, error) {
	namespace, err := r.destinationNamespace(ctx, request)
	if err != nil {
		return nil, err
	}

	common, err := newRequestCommon(request.SourceStorage, namespace)
	if err != nil {
		return nil, err
	}
	common.SetList = strings.Join(request.Policy.SetList, ",")

	// A limit the policy leaves unset is zero, which the request omits: the server's default applies.
	jobID := newJobID(time.Now())
	infoClient := client.InfoClient()
	err = infoClient.StartRestore(ctx, &infomodels.RequestRestore{
		RequestCommon:        common,
		JobID:                jobID,
		BackupIDs:            request.BackupDataPath,
		RestoreWorkerThreads: ptr.ValueOrZero(request.Policy.Parallel),
		RestoreRPS:           ptr.ValueOrZero(request.Policy.Tps),
		ExtraTTL:             int(ptr.ValueOrZero(request.Policy.ExtraTTL)),
		Fuzzy:                &infomodels.RequestFuzzyRestore{},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start server restore: %w", err)
	}

	return newRestoreHandler(infoClient, namespace, jobID), nil
}

// destinationNamespace returns the namespace to restore into: the mapped destination, or
// else the namespace the backup was taken from.
func (r *restoreExecutor) destinationNamespace(ctx context.Context, request *model.RestoreRequest) (string, error) {
	if request.Policy.Namespace != nil && request.Policy.Namespace.Destination != "" {
		return request.Policy.Namespace.Destination, nil
	}

	backups, err := r.backupReader.GetBackups(ctx, service.NewPathFilter(request.BackupDataPath, request.SourceStorage))
	if err != nil {
		return "", fmt.Errorf("failed to read server backup %s: %w", request.BackupDataPath, err)
	}
	if len(backups) == 0 {
		return "", fmt.Errorf("server backup %s not found", request.BackupDataPath)
	}

	return backups[0].Namespace, nil
}

// restoreHandler follows a server restore job.
type restoreHandler struct {
	infoClient backup.ServerBackupInfo
	namespace  string
	jobID      string
	stats      *models.RestoreStats
}

var _ restoreexecutor.RestoreHandler = (*restoreHandler)(nil)

func newRestoreHandler(infoClient backup.ServerBackupInfo, namespace, jobID string) *restoreHandler {
	stats := models.NewRestoreStats()
	stats.Start()

	return &restoreHandler{
		infoClient: infoClient,
		namespace:  namespace,
		jobID:      jobID,
		stats:      stats,
	}
}

// Wait polls the restore job until it ends or ctx is canceled.
func (h *restoreHandler) Wait(ctx context.Context) error {
	defer h.stats.Stop()

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
