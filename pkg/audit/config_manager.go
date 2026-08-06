package audit

import (
	"context"
	"log/slog"

	"github.com/aerospike/aerospike-backup-service/v3/internal/server/handlers"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
)

type auditConfigManager struct {
	underlying handlers.ConfigManager
	auditor    Auditor
}

var _ handlers.ConfigManager = (*auditConfigManager)(nil)

// NewAuditConfigManager wraps an existing ConfigManager with audit logging.
func NewAuditConfigManager(underlying handlers.ConfigManager, auditor Auditor) handlers.ConfigManager {
	return &auditConfigManager{
		underlying: underlying,
		auditor:    auditor,
	}
}

// ChangeBackupConfig delegates to the underlying manager and audits the operation.
func (a *auditConfigManager) ChangeBackupConfig(
	ctx context.Context,
	action string,
	resourceID string,
	mutate func(*dto.Config) ([]string, error),
	opts ...func(*handlers.BackupConfigChangeOptions),
) error {
	err := a.underlying.ChangeBackupConfig(ctx, action, resourceID, mutate, opts...)

	status := StatusSuccess
	if err != nil {
		status = StatusFailed
	}

	a.auditor.WriteEvent(ctx, action, status, slog.String("resource", resourceID))

	return err
}

func (a *auditConfigManager) UpdateConfig(ctx context.Context, newConfig *dto.Config) error {
	err := a.underlying.UpdateConfig(ctx, newConfig)
	status := StatusSuccess
	if err != nil {
		status = StatusFailed
	}
	a.auditor.WriteEvent(ctx, "UpdateConfig", status)
	return err
}

func (a *auditConfigManager) ApplyConfig(ctx context.Context) error {
	err := a.underlying.ApplyConfig(ctx)
	status := StatusSuccess
	if err != nil {
		status = StatusFailed
	}
	a.auditor.WriteEvent(ctx, "ApplyConfig", status)
	return err
}
