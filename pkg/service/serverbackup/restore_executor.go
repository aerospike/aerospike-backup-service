package serverbackup

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/restoreexecutor"
	secrets "github.com/aerospike/aerospike-backup-service/v3/pkg/service/secret"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/models"
	infomodels "github.com/aerospike/backup-go/pkg/asinfo/models"
)

// restoreExecutor asks the server to run a fuzzy restore of a server backup, identified by its job id.
type restoreExecutor struct {
	resolver secrets.Resolver
}

var _ restoreexecutor.Restore = (*restoreExecutor)(nil)

// NewRestoreExecutor returns a Restore that runs server-side restores.
func NewRestoreExecutor(resolver secrets.Resolver) restoreexecutor.Restore {
	return &restoreExecutor{resolver: resolver}
}

// Run starts the server restore of the job id in request.BackupDataPath.
func (r *restoreExecutor) Run(
	ctx context.Context,
	client aerospike.Client,
	request *model.RestoreRequest,
) (restoreexecutor.RestoreHandler, error) {
	namespace, err := destinationNamespace(request)
	if err != nil {
		return nil, err
	}

	common, err := newRequestCommon(ctx, r.resolver, request.SourceStorage, namespace)
	if err != nil {
		return nil, err
	}

	infoClient := client.InfoClient()
	err = infoClient.StartRestore(ctx, &infomodels.RequestRestore{
		RequestCommon: common,
		JobID:         request.BackupDataPath,
		FuzzyRestore:  true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start server restore: %w", err)
	}

	return newRestoreHandler(infoClient, namespace), nil
}

// destinationNamespace returns the namespace to restore into: the mapped destination, or the
// source namespace when it is not renamed.
func destinationNamespace(request *model.RestoreRequest) (string, error) {
	if request.Policy.Namespace == nil {
		return "", errors.New("server restore requires the namespace in the restore policy")
	}

	if destination := request.Policy.Namespace.Destination; destination != "" {
		return destination, nil
	}

	if source := request.Policy.Namespace.Source; source != "" {
		return source, nil
	}

	return "", errors.New("server restore requires the namespace in the restore policy")
}

// restoreHandler follows a server restore of one namespace.
type restoreHandler struct {
	infoClient backup.ServerBackupInfo
	namespace  string
	stats      *models.RestoreStats
}

var _ restoreexecutor.RestoreHandler = (*restoreHandler)(nil)

func newRestoreHandler(infoClient backup.ServerBackupInfo, namespace string) *restoreHandler {
	stats := models.NewRestoreStats()
	stats.Start()

	return &restoreHandler{
		infoClient: infoClient,
		namespace:  namespace,
		stats:      stats,
	}
}

// Wait polls the namespace restore state until the restore ends or ctx is canceled.
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

		state, err := h.infoClient.GetRestoreStatus(ctx, h.namespace)
		if err != nil {
			return fmt.Errorf("failed to get server restore status: %w", err)
		}

		switch state {
		case infomodels.RestoreStateNone:
			return nil
		case infomodels.RestoreStateFailed:
			return fmt.Errorf("server restore of namespace %s failed", h.namespace)
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
