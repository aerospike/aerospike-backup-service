package dto

import (
	"errors"
	"fmt"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// RestoreRequest represents a restore operation request from custom storage
// @Description RestoreRequest represents a restore operation request.
type RestoreRequest struct {
	DestinationClusterConfig
	*SecretAgentConfig
	StorageConfig
	// Restore policy to use in the operation.
	Policy *RestorePolicy `json:"policy"`
	// Path to the data from storage root.
	BackupDataPath string `json:"backup-data-path" validate:"required"`
}

// RestoreTimestampRequest represents a restore by timestamp operation request.
// @Description RestoreTimestampRequest represents a restore by timestamp operation request.
type RestoreTimestampRequest struct {
	DestinationClusterConfig
	*SecretAgentConfig
	StorageConfig
	// Restore policy to use in the operation.
	Policy *RestorePolicy `json:"policy"`
	// Required epoch time for recovery. The closest backup before the timestamp will be applied.
	Time int64 `json:"time,omitempty" format:"int64" example:"1739538000000" validate:"required"`
	// The backup routine name.
	Routine string `json:"routine,omitempty" example:"daily" validate:"required"`
}

// Validate validates the restore operation request.
func (r *RestoreRequest) Validate() error {
	if len(r.BackupDataPath) == 0 {
		return errors.New("path is not specified")
	}
	if err := r.DestinationClusterConfig.Validate(); err != nil {
		return err
	}
	if err := r.Policy.Validate(); err != nil {
		return err
	}
	if err := r.StorageConfig.Validate(); err != nil {
		return err
	}
	if err := r.Policy.Validate(); err != nil {
		return err
	}

	return nil
}

// Validate validates the restore operation request.
func (r *RestoreTimestampRequest) Validate() error {
	if err := r.DestinationClusterConfig.Validate(); err != nil {
		return err
	}
	if err := r.Policy.Validate(); err != nil {
		return err
	}
	if r.Time <= 0 {
		return errors.New("restore point in time should be positive")
	}
	if r.Routine == "" {
		return errValidationEmptyField("routine")
	}

	return nil
}

func (r *RestoreTimestampRequest) ToModel(config *model.Config) (*model.RestoreTimestampRequest, error) {
	cluster, err := r.DestinationClusterConfig.ToModel(config)
	if err != nil {
		return nil, fmt.Errorf("invalid cluster: %w", err)
	}
	if _, ok := config.Routines()[r.Routine]; !ok {
		return nil, errValidationNotFound("routine", r.Routine)
	}

	secretAgent, err := r.SecretAgentConfig.ToModel(config)
	if err != nil {
		return nil, fmt.Errorf("invalid secret agent: %w", err)
	}

	return &model.RestoreTimestampRequest{
		DestinationCluster: cluster,
		Policy:             r.Policy.ToModel(),
		SecretAgent:        secretAgent,
		Time:               time.UnixMilli(r.Time),
		RoutineName:        r.Routine,
	}, nil
}

func (r *RestoreRequest) ToModel(config *model.Config) (*model.RestoreRequest, error) {
	cluster, err := r.DestinationClusterConfig.ToModel(config)
	if err != nil {
		return nil, fmt.Errorf("invalid cluster: %w", err)
	}

	storage, err := r.StorageConfig.ToModel(config)
	if err != nil {
		return nil, fmt.Errorf("invalid storage: %w", err)
	}

	secretAgent, err := r.SecretAgentConfig.ToModel(config)
	if err != nil {
		return nil, fmt.Errorf("invalid secret-agent: %w", err)
	}

	return &model.RestoreRequest{
		DestinationCluster: cluster,
		Policy:             r.Policy.ToModel(),
		SourceStorage:      storage,
		SecretAgent:        secretAgent,
		BackupDataPath:     r.BackupDataPath,
	}, nil
}

// DestinationClusterConfig aggregates the destination cluster configuration.
type DestinationClusterConfig struct {
	// The details of the Aerospike destination cluster.
	// Mutually exclusive with 'destination-name'.
	Cluster *AerospikeCluster `json:"destination,omitempty"`
	// Link to one of preconfigured clusters.
	// Mutually exclusive with 'destination'.
	Name *string `json:"destination-name,omitempty"`
}

func (c *DestinationClusterConfig) Validate() error {
	if c.Cluster == nil && c.Name == nil {
		return errValidationRequiredField("destination", "destination-name")
	}
	if c.Cluster != nil && c.Name != nil {
		return errValidationMutuallyExclusive("destination", "destination-name")
	}
	if c.Cluster != nil {
		if err := c.Cluster.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func (c *DestinationClusterConfig) ToModel(config *model.Config) (*model.AerospikeCluster, error) {
	if c.Cluster != nil {
		return c.Cluster.ToModel(config)
	}

	configCluster, exists := config.BackupConfigCopy().AerospikeClusters[*c.Name]
	if !exists {
		return nil, errValidationNotFound("cluster", *c.Name)
	}

	return configCluster, nil
}

// StorageConfig aggregates the storage configuration.
type StorageConfig struct {
	// The details of the storage configuration.
	// Mutually exclusive with 'source-name'.
	Storage *Storage `json:"source,omitempty"`
	// Link to one of preconfigured storages.
	// Mutually exclusive with 'source'.
	Name *string `json:"source-name,omitempty"`
}

func (c *StorageConfig) Validate() error {
	if c.Storage == nil && c.Name == nil {
		return errValidationRequiredField("source", "source-name")
	}
	if c.Storage != nil && c.Name != nil {
		return errValidationMutuallyExclusive("source", "source-name")
	}
	if c.Storage != nil {
		if err := c.Storage.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func (c *StorageConfig) ToModel(config *model.Config) (model.Storage, error) {
	if c.Storage != nil {
		return c.Storage.ToModel(config)
	}

	configStorage, exists := config.BackupConfigCopy().Storage[*c.Name]
	if !exists {
		return nil, errValidationNotFound("storage", *c.Name)
	}
	return configStorage, nil
}
