package dto

import (
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
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
	// You can obtain this value by:
	// - Browsing the storage UI, or
	// - Reading the `key` field in the response from GET `v1/backups/full/{routine}`
	BackupDataPath string `json:"backup-data-path" validate:"required"`
}

// NewRestoreRequestFromReader reads and deserializes the restore request from reader.
func NewRestoreRequestFromReader(r io.Reader) (*RestoreRequest, error) {
	var req RestoreRequest
	err := decoder.Deserialize(&req, r, decoder.JSON)
	if err != nil {
		return nil, err
	}

	return &req, nil
}

// RestoreTimestampRequest represents a restore by timestamp operation request.
// @Description RestoreTimestampRequest represents a restore by timestamp operation request.
type RestoreTimestampRequest struct {
	DestinationClusterConfig
	*SecretAgentConfig
	// Restore policy to use in the operation.
	Policy *TimestampRestorePolicy `json:"policy,omitempty"`
	// Required epoch time (in millis) for recovery. The closest backup before the timestamp will be applied.
	Time int64 `json:"time" format:"int64" example:"1739538000000" validate:"required" minimum:"1000000000000"`
	// The backup routine name.
	Routine string `json:"routine" example:"daily" validate:"required"`
	// Disable reverse order of incremental backups optimisation.
	DisableReordering bool `json:"disable-reordering,omitempty" default:"false"`
}

// NewRestoreTimestampRequestFromReader reads and deserializes the restore by timestamp request from reader.
func NewRestoreTimestampRequestFromReader(r io.Reader) (*RestoreTimestampRequest, error) {
	var req RestoreTimestampRequest
	err := decoder.Deserialize(&req, r, decoder.JSON)
	if err != nil {
		return nil, err
	}
	return &req, nil
}

// Validate validates the restore operation request.
func (r *RestoreRequest) Validate(opts ...ValidationOption) error {
	if len(r.BackupDataPath) == 0 {
		return errValidationEmptyField("backup-data-path")
	}
	if err := r.DestinationClusterConfig.Validate(opts...); err != nil {
		return err
	}
	if err := r.Policy.Validate(); err != nil {
		return err
	}
	if err := r.StorageConfig.Validate(opts...); err != nil {
		return err
	}

	return nil
}

// Validate validates the restore operation request.
func (r *RestoreTimestampRequest) Validate(opts ...ValidationOption) error {
	// If no cluster is specified, routine's cluster will be used.
	if err := r.DestinationClusterConfig.Validate(append(opts, ValidationAllowEmpty)...); err != nil {
		return err
	}
	if err := r.Policy.Validate(); err != nil {
		return err
	}
	if r.Time == 0 {
		return errValidationEmptyField("time")
	}
	if r.Time < 0 {
		return errValidationNegative("time", r.Time)
	}
	if r.Time < 1_000_000_000_000 { // 13 digits for unix timestamp
		return fmt.Errorf("%w: restore timestamp must be in milliseconds", errValidation)
	}
	if r.Routine == "" {
		return errValidationEmptyField("routine")
	}

	return nil
}

func (r *RestoreTimestampRequest) ToModel(config *model.Config) (*model.RestoreTimestampRequest, error) {
	routine, found := config.Routine(r.Routine)
	if !found {
		return nil, errValidationNotFound("routine", r.Routine)
	}

	cluster, err := r.DestinationClusterConfig.ToModel(config)
	if err != nil {
		return nil, fmt.Errorf("invalid cluster: %w", err)
	}

	if cluster == nil { // if cluster is not specified, use routine's cluster.
		cluster = routine.SourceCluster
	}

	secretAgent, err := r.SecretAgentConfig.ToModel(config)
	if err != nil {
		return nil, fmt.Errorf("invalid secret agent: %w", err)
	}
	if secretAgent == nil {
		secretAgent = routine.SecretAgent // use routine's secret agent if not specified.
	}

	return &model.RestoreTimestampRequest{
		DestinationCluster: *cluster,
		Policy:             *r.Policy.ToModel(),
		SecretAgent:        secretAgent,
		Time:               time.UnixMilli(r.Time),
		Routine:            routine.Copy(), // restore will work on it's own copy.
		DisableReordering:  r.DisableReordering,
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
		DestinationCluster: *cluster,
		Policy:             *r.Policy.ToModel(),
		SourceStorage:      storage,
		SecretAgent:        secretAgent,
		BackupDataPath:     r.BackupDataPath,
	}, nil
}

// DestinationClusterConfig aggregates the destination cluster configuration.
// It is intended to be embedded into DTOs that require Cluster configuration.
type DestinationClusterConfig struct {
	// The details of the Aerospike destination cluster.
	// Mutually exclusive with 'destination-name'.
	Cluster *AerospikeCluster `json:"destination,omitempty"`
	// Link to one of preconfigured clusters.
	// Mutually exclusive with 'destination'.
	Name string `json:"destination-name,omitempty" extensions:"x-nullable"`
}

func (c *DestinationClusterConfig) IsEmpty() bool {
	return c.Name == "" && c.Cluster == nil
}

func (c *DestinationClusterConfig) Validate(opts ...ValidationOption) error {
	if c.Cluster == nil && c.Name == "" && !slices.Contains(opts, ValidationAllowEmpty) {
		return errValidationRequiredEither("destination", "destination-name")
	}
	if c.Cluster != nil && c.Name != "" {
		return errValidationMutuallyExclusive("destination", "destination-name")
	}
	if c.Cluster != nil {
		if err := c.Cluster.Validate(opts...); err != nil {
			return err
		}
	}

	return nil
}

func (c *DestinationClusterConfig) ToModel(config *model.Config) (*model.AerospikeCluster, error) {
	if c.IsEmpty() {
		return nil, nil
	}

	if c.Cluster != nil {
		return c.Cluster.ToModel(config)
	}

	configCluster, exists := config.BackupConfigCopy().AerospikeClusters[c.Name]
	if !exists {
		return nil, errValidationNotFound("cluster", c.Name)
	}

	return configCluster, nil
}

// StorageConfig aggregates the storage configuration.
// It is intended to be embedded into DTOs that require Storage configuration.
type StorageConfig struct {
	// The details of the storage configuration.
	// Mutually exclusive with 'source-name'.
	Storage *Storage `json:"source,omitempty"`
	// Link to one of preconfigured storages.
	// Mutually exclusive with 'source'.
	Name string `json:"source-name,omitempty" extensions:"x-nullable"`
}

func (c *StorageConfig) Validate(opts ...ValidationOption) error {
	if c.Storage == nil && c.Name == "" {
		return errValidationRequiredEither("source", "source-name")
	}
	if c.Storage != nil && c.Name != "" {
		return errValidationMutuallyExclusive("source", "source-name")
	}
	if c.Storage != nil {
		if err := c.Storage.Validate(opts...); err != nil {
			return err
		}
	}

	return nil
}

func (c *StorageConfig) ToModel(config *model.Config) (model.Storage, error) {
	if c.Storage != nil {
		return c.Storage.ToModel(config)
	}

	configStorage, exists := config.BackupConfigCopy().Storage[c.Name]
	if !exists {
		return nil, errValidationNotFound("storage", c.Name)
	}
	return configStorage, nil
}
