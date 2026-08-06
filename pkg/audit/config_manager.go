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

func (a *auditConfigManager) ReadConfig(ctx context.Context) *dto.Config {
	return a.underlying.ReadConfig(ctx)
}

func (a *auditConfigManager) ReadAerospikeClusters(ctx context.Context) map[string]*dto.AerospikeCluster {
	return a.underlying.ReadAerospikeClusters(ctx)
}

func (a *auditConfigManager) ReadAerospikeCluster(ctx context.Context, name string) (*dto.AerospikeCluster, error) {
	return a.underlying.ReadAerospikeCluster(ctx, name)
}

func (a *auditConfigManager) AddAerospikeCluster(
	ctx context.Context, name string, cluster *dto.AerospikeCluster,
) error {
	err := a.underlying.AddAerospikeCluster(ctx, name, cluster)
	status := StatusSuccess
	if err != nil {
		status = StatusFailed
	}
	a.auditor.WriteEvent(ctx, "AddAerospikeCluster", status, slog.String("resource", name))
	return err
}

func (a *auditConfigManager) UpdateAerospikeCluster(
	ctx context.Context, name string, cluster *dto.AerospikeCluster,
) error {
	err := a.underlying.UpdateAerospikeCluster(ctx, name, cluster)
	status := StatusSuccess
	if err != nil {
		status = StatusFailed
	}
	a.auditor.WriteEvent(ctx, "UpdateAerospikeCluster", status, slog.String("resource", name))
	return err
}

func (a *auditConfigManager) DeleteAerospikeCluster(ctx context.Context, name string) error {
	err := a.underlying.DeleteAerospikeCluster(ctx, name)
	status := StatusSuccess
	if err != nil {
		status = StatusFailed
	}
	a.auditor.WriteEvent(ctx, "DeleteAerospikeCluster", status, slog.String("resource", name))
	return err
}

func (a *auditConfigManager) ReadPolicies(ctx context.Context) map[string]*dto.BackupPolicy {
	return a.underlying.ReadPolicies(ctx)
}

func (a *auditConfigManager) ReadPolicy(ctx context.Context, name string) (*dto.BackupPolicy, error) {
	return a.underlying.ReadPolicy(ctx, name)
}

func (a *auditConfigManager) AddPolicy(ctx context.Context, name string, policy *dto.BackupPolicy) error {
	err := a.underlying.AddPolicy(ctx, name, policy)
	status := StatusSuccess
	if err != nil {
		status = StatusFailed
	}
	a.auditor.WriteEvent(ctx, "AddPolicy", status, slog.String("resource", name))
	return err
}

func (a *auditConfigManager) UpdatePolicy(ctx context.Context, name string, policy *dto.BackupPolicy) error {
	err := a.underlying.UpdatePolicy(ctx, name, policy)
	status := StatusSuccess
	if err != nil {
		status = StatusFailed
	}
	a.auditor.WriteEvent(ctx, "UpdatePolicy", status, slog.String("resource", name))
	return err
}

func (a *auditConfigManager) DeletePolicy(ctx context.Context, name string) error {
	err := a.underlying.DeletePolicy(ctx, name)
	status := StatusSuccess
	if err != nil {
		status = StatusFailed
	}
	a.auditor.WriteEvent(ctx, "DeletePolicy", status, slog.String("resource", name))
	return err
}

func (a *auditConfigManager) ReadRoutines(ctx context.Context) map[string]*dto.BackupRoutine {
	return a.underlying.ReadRoutines(ctx)
}

func (a *auditConfigManager) ReadRoutine(ctx context.Context, name string) (*dto.BackupRoutine, error) {
	return a.underlying.ReadRoutine(ctx, name)
}

func (a *auditConfigManager) AddRoutine(ctx context.Context, name string, routine *dto.BackupRoutine) error {
	err := a.underlying.AddRoutine(ctx, name, routine)
	status := StatusSuccess
	if err != nil {
		status = StatusFailed
	}
	a.auditor.WriteEvent(ctx, "AddRoutine", status, slog.String("resource", name))
	return err
}

func (a *auditConfigManager) UpdateRoutine(ctx context.Context, name string, routine *dto.BackupRoutine) error {
	err := a.underlying.UpdateRoutine(ctx, name, routine)
	status := StatusSuccess
	if err != nil {
		status = StatusFailed
	}
	a.auditor.WriteEvent(ctx, "UpdateRoutine", status, slog.String("resource", name))
	return err
}

func (a *auditConfigManager) DeleteRoutine(ctx context.Context, name string) error {
	err := a.underlying.DeleteRoutine(ctx, name)
	status := StatusSuccess
	if err != nil {
		status = StatusFailed
	}
	a.auditor.WriteEvent(ctx, "DeleteRoutine", status, slog.String("resource", name))
	return err
}

func (a *auditConfigManager) EnableRoutine(ctx context.Context, name string) error {
	err := a.underlying.EnableRoutine(ctx, name)
	status := StatusSuccess
	if err != nil {
		status = StatusFailed
	}
	a.auditor.WriteEvent(ctx, "EnableRoutine", status, slog.String("resource", name))
	return err
}

func (a *auditConfigManager) DisableRoutine(ctx context.Context, name string) error {
	err := a.underlying.DisableRoutine(ctx, name)
	status := StatusSuccess
	if err != nil {
		status = StatusFailed
	}
	a.auditor.WriteEvent(ctx, "DisableRoutine", status, slog.String("resource", name))
	return err
}

func (a *auditConfigManager) ReadAllStorage(ctx context.Context) map[string]*dto.Storage {
	return a.underlying.ReadAllStorage(ctx)
}

func (a *auditConfigManager) ReadStorage(ctx context.Context, name string) (*dto.Storage, error) {
	return a.underlying.ReadStorage(ctx, name)
}

func (a *auditConfigManager) AddStorage(ctx context.Context, name string, storage *dto.Storage) error {
	err := a.underlying.AddStorage(ctx, name, storage)
	status := StatusSuccess
	if err != nil {
		status = StatusFailed
	}
	a.auditor.WriteEvent(ctx, "AddStorage", status, slog.String("resource", name))
	return err
}

func (a *auditConfigManager) UpdateStorage(ctx context.Context, name string, storage *dto.Storage) error {
	err := a.underlying.UpdateStorage(ctx, name, storage)
	status := StatusSuccess
	if err != nil {
		status = StatusFailed
	}
	a.auditor.WriteEvent(ctx, "UpdateStorage", status, slog.String("resource", name))
	return err
}

func (a *auditConfigManager) DeleteStorage(ctx context.Context, name string) error {
	err := a.underlying.DeleteStorage(ctx, name)
	status := StatusSuccess
	if err != nil {
		status = StatusFailed
	}
	a.auditor.WriteEvent(ctx, "DeleteStorage", status, slog.String("resource", name))
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
