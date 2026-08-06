package audit

import (
	"context"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
)

type auditConfigRetriever struct {
	underlying service.ConfigRetriever
}

var _ service.ConfigRetriever = (*auditConfigRetriever)(nil)

// NewAuditConfigRetriever wraps an existing ConfigRetriever to allow future security/RBAC additions.
func NewAuditConfigRetriever(underlying service.ConfigRetriever) service.ConfigRetriever {
	return &auditConfigRetriever{
		underlying: underlying,
	}
}

func (a *auditConfigRetriever) RetrieveConfiguration(
	ctx context.Context,
	routine *model.BackupRoutine,
	timestamp time.Time,
) ([]byte, error) {
	return a.underlying.RetrieveConfiguration(ctx, routine, timestamp)
}
