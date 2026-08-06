package audit

import (
	"context"
	"log/slog"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
)

type auditRestoreManager struct {
	underlying service.RestoreManager
	auditor    Auditor
}

var _ service.RestoreManager = (*auditRestoreManager)(nil)

// NewAuditRestoreManager wraps an existing RestoreManager with audit logging.
func NewAuditRestoreManager(underlying service.RestoreManager, auditor Auditor) service.RestoreManager {
	return &auditRestoreManager{
		underlying: underlying,
		auditor:    auditor,
	}
}

func (a *auditRestoreManager) Restore(ctx context.Context, request *model.RestoreRequest) (model.RestoreJobID, error) {
	jobID, err := a.underlying.Restore(ctx, request)

	status := StatusSuccess
	if err != nil {
		status = StatusFailed
	}
	a.auditor.WriteEvent(ctx, "RestoreStarted", status, slog.Int64("jobId", int64(jobID)))

	return jobID, err
}

func (a *auditRestoreManager) RestoreByTime(
	ctx context.Context, request *model.RestoreTimestampRequest,
) (model.RestoreJobID, error) {
	jobID, err := a.underlying.RestoreByTime(ctx, request)

	status := StatusSuccess
	if err != nil {
		status = StatusFailed
	}
	a.auditor.WriteEvent(ctx, "RestoreByTimeStarted", status, slog.Int64("jobId", int64(jobID)))

	return jobID, err
}

func (a *auditRestoreManager) JobStatus(jobID model.RestoreJobID) (*model.RestoreJobStatus, error) {
	return a.underlying.JobStatus(jobID)
}

func (a *auditRestoreManager) CancelRestore(jobID model.RestoreJobID) error {
	err := a.underlying.CancelRestore(jobID)

	status := StatusSuccess
	if err != nil {
		status = StatusFailed
	}
	a.auditor.WriteEvent(context.Background(), "CancelRestore", status, slog.Int64("jobId", int64(jobID)))

	return err
}

func (a *auditRestoreManager) GetFilteredJobs(
	timeBounds model.TimeBounds,
	statusFilter model.StatusFilter,
) map[model.RestoreJobID]*model.RestoreJobStatus {
	return a.underlying.GetFilteredJobs(timeBounds, statusFilter)
}
