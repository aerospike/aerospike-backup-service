package audit

import (
	"context"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
)

type auditBackupReader struct {
	underlying service.BackupReader
}

var _ service.BackupReader = (*auditBackupReader)(nil)

// NewAuditBackupReader wraps an existing BackupReader to allow future security/RBAC additions.
// Currently it just passes through as read-only operations don't emit audit logs by default.
func NewAuditBackupReader(underlying service.BackupReader) service.BackupReader {
	return &auditBackupReader{
		underlying: underlying,
	}
}

func (a *auditBackupReader) GetBackups(
	ctx context.Context, filter service.BackupFilter,
) ([]model.BackupDetails, error) {
	return a.underlying.GetBackups(ctx, filter)
}
