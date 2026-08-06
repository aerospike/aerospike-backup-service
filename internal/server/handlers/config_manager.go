package handlers

import (
	"context"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
)

// ConfigManager defines the interface for mutating configuration state.
// By isolating this in an interface, we guarantee that all config changes
// can be intercepted for Auditing and future RBAC validation.
type ConfigManager interface {
	// ChangeBackupConfig applies a mutation to the backup configuration DTO,
	// validates the full configuration via ToModel, and persists the result.
	// action: e.g. "AddAerospikeCluster"
	// resourceID: e.g. cluster name
	ChangeBackupConfig(
		ctx context.Context,
		action string,
		resourceID string,
		mutate func(*dto.Config) ([]string, error),
		opts ...func(*BackupConfigChangeOptions),
	) error

	// UpdateConfig updates the configuration for the service.
	UpdateConfig(ctx context.Context, newConfig *dto.Config) error

	// ApplyConfig reloads the configuration from the config file.
	ApplyConfig(ctx context.Context) error
}
