package dto

import (
	"errors"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
)

// RestoreRequest represents a restore operation request.
// @Description RestoreRequest represents a restore operation request.
type RestoreRequest struct {
	DestinationCuster *AerospikeCluster `json:"destination,omitempty" validate:"required"`
	Policy            *RestorePolicy    `json:"policy,omitempty" validate:"required"`
	SourceStorage     *Storage          `json:"source,omitempty" validate:"required"`
	SecretAgent       *SecretAgent      `json:"secret-agent,omitempty"`
}

// RestoreRequestInternal is used internally to prepopulate data for the restore operation.
type RestoreRequestInternal struct {
	RestoreRequest
	Dir *string
}

// RestoreTimestampRequest represents a restore by timestamp operation request.
// @Description RestoreTimestampRequest represents a restore by timestamp operation request.
type RestoreTimestampRequest struct {
	// The details of the Aerospike destination cluster.
	DestinationCuster *AerospikeCluster `json:"destination,omitempty" validate:"required"`
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
	if err := r.DestinationCuster.Validate(); err != nil {
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
func (r *RestoreTimestampRequest) Validate(config *model.Config) error {
	if err := r.DestinationCuster.Validate(); err != nil {
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
	if _, ok := config.BackupRoutines[r.Routine]; !ok {
		return notFoundValidationError("routine", r.Routine)
	}
	return nil
}

func (r RestoreTimestampRequest) ToModel() *model.RestoreTimestampRequest {
	return &model.RestoreTimestampRequest{
		DestinationCuster: r.DestinationCuster.ToModel(),
		Policy:            r.Policy.ToModel(),
		SecretAgent:       r.SecretAgent.ToModel(),
		Time:              time.UnixMilli(r.Time),
		Routine:           r.Routine,
	}
}

func (r RestoreRequestInternal) ToModel() *model.RestoreRequestInternal {
	return &model.RestoreRequestInternal{
		RestoreRequest: model.RestoreRequest{
			DestinationCuster: r.DestinationCuster.ToModel(),
			Policy:            r.Policy.ToModel(),
			SourceStorage:     r.SourceStorage.ToModel(),
			SecretAgent:       r.SecretAgent.ToModel(),
		},
		Dir: r.Dir,
	}
}

func (r *RestoreRequest) ToModel() *model.RestoreRequest {
	return &model.RestoreRequest{
		DestinationCuster: r.DestinationCuster.ToModel(),
		Policy:            r.Policy.ToModel(),
		SourceStorage:     r.SourceStorage.ToModel(),
		SecretAgent:       r.SecretAgent.ToModel(),
	}
}
