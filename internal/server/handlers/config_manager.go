package handlers

import (
	"context"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
)

type BackupConfigChangeOptions struct {
	validateNamespaces bool
}

// ConfigManager defines the interface for mutating configuration state.
// By isolating this in an interface, we guarantee that all config changes
// can be intercepted for Auditing and future RBAC validation.
type ConfigManager interface {
	// UpdateConfig updates the configuration for the service.
	UpdateConfig(ctx context.Context, newConfig *dto.Config) error
	// ApplyConfig reloads the configuration from the config file.
	ApplyConfig(ctx context.Context) error

	// ReadConfig returns the service configuration DTO.
	ReadConfig(ctx context.Context) *dto.Config

	// ReadAerospikeClusters reads all Aerospike clusters from the configuration.
	ReadAerospikeClusters(ctx context.Context) map[string]*dto.AerospikeCluster
	// AddAerospikeCluster adds a new Aerospike cluster.
	AddAerospikeCluster(ctx context.Context, name string, cluster *dto.AerospikeCluster) error
	// ReadAerospikeCluster reads a specific Aerospike cluster by name.
	ReadAerospikeCluster(ctx context.Context, name string) (*dto.AerospikeCluster, error)
	// UpdateAerospikeCluster updates an existing Aerospike cluster.
	UpdateAerospikeCluster(ctx context.Context, name string, cluster *dto.AerospikeCluster) error
	// DeleteAerospikeCluster deletes an Aerospike cluster by name.
	DeleteAerospikeCluster(ctx context.Context, name string) error

	// ReadPolicies reads all backup policies from the configuration.
	ReadPolicies(ctx context.Context) map[string]*dto.BackupPolicy
	// AddPolicy adds a new backup policy.
	AddPolicy(ctx context.Context, name string, policy *dto.BackupPolicy) error
	// ReadPolicy reads a specific backup policy by name.
	ReadPolicy(ctx context.Context, name string) (*dto.BackupPolicy, error)
	// UpdatePolicy updates an existing backup policy.
	UpdatePolicy(ctx context.Context, name string, policy *dto.BackupPolicy) error
	// DeletePolicy deletes a backup policy by name.
	DeletePolicy(ctx context.Context, name string) error

	// ReadRoutines reads all backup routines from the configuration.
	ReadRoutines(ctx context.Context) map[string]*dto.BackupRoutine
	// AddRoutine adds a new backup routine.
	AddRoutine(ctx context.Context, name string, routine *dto.BackupRoutine) error
	// ReadRoutine reads a specific backup routine by name.
	ReadRoutine(ctx context.Context, name string) (*dto.BackupRoutine, error)
	// UpdateRoutine updates an existing backup routine.
	UpdateRoutine(ctx context.Context, name string, routine *dto.BackupRoutine) error
	// DeleteRoutine deletes a backup routine by name.
	DeleteRoutine(ctx context.Context, name string) error
	// EnableRoutine enables a disabled backup routine.
	EnableRoutine(ctx context.Context, name string) error
	// DisableRoutine disables an active backup routine.
	DisableRoutine(ctx context.Context, name string) error

	// ReadAllStorage reads all storage configurations.
	ReadAllStorage(ctx context.Context) map[string]*dto.Storage
	// AddStorage adds a new storage configuration.
	AddStorage(ctx context.Context, name string, storage *dto.Storage) error
	// ReadStorage reads a specific storage configuration by name.
	ReadStorage(ctx context.Context, name string) (*dto.Storage, error)
	// UpdateStorage updates an existing storage configuration.
	UpdateStorage(ctx context.Context, name string, storage *dto.Storage) error
	// DeleteStorage deletes a storage configuration by name.
	DeleteStorage(ctx context.Context, name string) error
}
