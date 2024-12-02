package dto

import (
	"errors"
	"fmt"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/util"
)

// RestoreRequest represents a restore operation request from custom storage
// @Description RestoreRequest represents a restore operation request.
type RestoreRequest struct {
	DestinationCluster *AerospikeCluster `json:"destination,omitempty" validate:"required"`
	Policy             *RestorePolicy    `json:"policy,omitempty" validate:"required"`
	SourceStorage      *Storage          `json:"source,omitempty" validate:"required"`
	SecretAgent        *SecretAgent      `json:"secret-agent,omitempty"`
	// Path to the data from storage root.
	BackupDataPath *string `json:"backup-data-path" validate:"required"`
}

// RestoreTimestampRequest represents a restore by timestamp operation request.
// @Description RestoreTimestampRequest represents a restore by timestamp operation request.
type RestoreTimestampRequest struct {
	// The details of the Aerospike destination cluster.
	DestinationCluster *AerospikeCluster `json:"destination,omitempty" validate:"required"`
	// Restore policy to use in the operation.
	Policy *RestorePolicy `json:"policy,omitempty" validate:"required"`
	// Secret Agent configuration (optional).
	SecretAgent *SecretAgent `json:"secret-agent,omitempty"`
	// Required epoch time for recovery. The closest backup before the timestamp will be applied.
	Time int64 `json:"time,omitempty" format:"int64" example:"1739538000000" validate:"required"`
	// The backup routine name.
	Routine string `json:"routine,omitempty" example:"daily" validate:"required"`
}

// Validate validates the restore operation request.
func (r *RestoreRequest) Validate() error {
	if r.BackupDataPath == nil {
		return errors.New("path is not specified")
	}
	if err := r.DestinationCluster.Validate(); err != nil {
		return err
	}
	if err := r.Policy.Validate(); err != nil {
		return err
	}
	if err := r.SourceStorage.Validate(); err != nil {
		return err
	}
	if err := r.Policy.Validate(); err != nil {
		return err
	}
	return nil
}

// Validate validates the restore operation request.
func (r *RestoreTimestampRequest) Validate() error {
	if err := r.DestinationCluster.Validate(); err != nil {
		return err
	}
	if err := r.Policy.Validate(); err != nil {
		return err
	}
	if r.Time <= 0 {
		return errors.New("restore point in time should be positive")
	}
	if r.Routine == "" {
		return emptyFieldValidationError(r.Routine)
	}
	return nil
}

func (r *RestoreTimestampRequest) ToModel(config *model.Config) (*model.RestoreTimestampRequest, error) {
	cluster, err := r.DestinationCluster.ToModel(config)
	if err != nil {
		return nil, fmt.Errorf("invalid cluster: %w", err)
	}
	if _, ok := config.BackupRoutines[r.Routine]; !ok {
		return nil, notFoundValidationError("routine", r.Routine)
	}

	return &model.RestoreTimestampRequest{
		DestinationCluster: cluster,
		Policy:             r.Policy.ToModel(),
		SecretAgent:        r.SecretAgent.ToModel(),
		Time:               time.UnixMilli(r.Time),
		RoutineName:        r.Routine,
	}, nil
}

func (r *RestoreRequest) ToModel(config *model.Config) (*model.RestoreRequest, error) {
	cluster, err := r.DestinationCluster.ToModel(config)
	if err != nil {
		return nil, fmt.Errorf("invalid cluster: %w", err)
	}

	return &model.RestoreRequest{
		DestinationCluster: cluster,
		Policy:             r.Policy.ToModel(),
		SourceStorage:      r.SourceStorage.ToModel(),
		SecretAgent:        r.SecretAgent.ToModel(),
		BackupDataPath:     util.ValueOrZero(r.BackupDataPath),
	}, nil
}
