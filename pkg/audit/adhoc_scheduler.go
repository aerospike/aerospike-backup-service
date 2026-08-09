package audit

import (
	"context"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
)

type auditAdHocScheduler struct {
	underlying service.AdHocScheduler
	auditor    Auditor
}

var _ service.AdHocScheduler = (*auditAdHocScheduler)(nil)

// NewAuditAdHocScheduler wraps an existing AdHocScheduler with audit logging.
func NewAuditAdHocScheduler(underlying service.AdHocScheduler, auditor Auditor) service.AdHocScheduler {
	return &auditAdHocScheduler{
		underlying: underlying,
		auditor:    auditor,
	}
}

func (a *auditAdHocScheduler) TriggerAdHocFullBackup(routine *model.BackupRoutine, delay time.Duration) error {
	err := a.underlying.TriggerAdHocFullBackup(routine, delay)
	a.auditor.WriteEvent(
		context.Background(),
		"TriggerAdHocFullBackup",
		EventStatusFromError(err),
		slog.String("routine", routine.Name),
	)
	return err
}

func (a *auditAdHocScheduler) TriggerAdHocIncrementalBackup(routine *model.BackupRoutine, delay time.Duration) error {
	err := a.underlying.TriggerAdHocIncrementalBackup(routine, delay)
	a.auditor.WriteEvent(
		context.Background(),
		"TriggerAdHocIncrementalBackup",
		EventStatusFromError(err),
		slog.String("routine", routine.Name),
	)
	return err
}
