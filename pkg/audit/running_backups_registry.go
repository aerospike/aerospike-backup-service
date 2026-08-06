package audit

import (
	"context"
	"log/slog"

	"github.com/aerospike/aerospike-backup-service/v3/internal/server/handlers"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

type auditRunningBackupsRegistry struct {
	underlying handlers.RunningBackupsRegistry
	auditor    Auditor
}

var _ handlers.RunningBackupsRegistry = (*auditRunningBackupsRegistry)(nil)

// NewAuditRunningBackupsRegistry wraps an existing RunningBackupsRegistry to allow future security/RBAC additions
// and emit audit logs for mutations like Cancel.
func NewAuditRunningBackupsRegistry(
	underlying handlers.RunningBackupsRegistry, auditor Auditor,
) handlers.RunningBackupsRegistry {
	return &auditRunningBackupsRegistry{
		underlying: underlying,
		auditor:    auditor,
	}
}

func (a *auditRunningBackupsRegistry) GetRoutineState(routine *model.BackupRoutine) model.RoutineState {
	return a.underlying.GetRoutineState(routine)
}

func (a *auditRunningBackupsRegistry) GetRunningState() map[string]model.RoutineState {
	return a.underlying.GetRunningState()
}

func (a *auditRunningBackupsRegistry) Cancel(routineName string) {
	a.underlying.Cancel(routineName)
	// Currently Cancel() in registry doesn't return error.
	a.auditor.WriteEvent(context.Background(), "CancelBackup", StatusSuccess, slog.String("routine", routineName))
}
