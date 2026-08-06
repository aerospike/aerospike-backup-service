package handlers

import (
	"context"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
)

// ConfigManager defines the interface for mutating configuration state.
// By isolating this in an interface, we guarantee that all config changes
// can be intercepted for Auditing and future RBAC validation.
type ConfigManager interface {
	UpdateConfig(ctx context.Context, newConfig *dto.Config) error
	ApplyConfig(ctx context.Context) error

	AddAerospikeCluster(ctx context.Context, name string, cluster *dto.AerospikeCluster) error
	UpdateAerospikeCluster(ctx context.Context, name string, cluster *dto.AerospikeCluster) error
	DeleteAerospikeCluster(ctx context.Context, name string) error

	AddPolicy(ctx context.Context, name string, policy *dto.BackupPolicy) error
	UpdatePolicy(ctx context.Context, name string, policy *dto.BackupPolicy) error
	DeletePolicy(ctx context.Context, name string) error

	AddRoutine(ctx context.Context, name string, routine *dto.BackupRoutine) error
	UpdateRoutine(ctx context.Context, name string, routine *dto.BackupRoutine) error
	DeleteRoutine(ctx context.Context, name string) error
	EnableRoutine(ctx context.Context, name string) error
	DisableRoutine(ctx context.Context, name string) error

	AddStorage(ctx context.Context, name string, storage *dto.Storage) error
	UpdateStorage(ctx context.Context, name string, storage *dto.Storage) error
	DeleteStorage(ctx context.Context, name string) error
}
