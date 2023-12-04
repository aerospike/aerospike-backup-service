package model

import (
	"encoding/json"
	"errors"
)

// RestoreRequest represents a restore operation request.
type RestoreRequest struct {
	DestinationCuster *AerospikeCluster `json:"destination,omitempty"`
	Policy            *RestorePolicy    `json:"policy,omitempty"`
	SourceStorage     *Storage          `json:"source,omitempty"`
}

// RestoreRequestInternal is used internally to prepopulate data for the restore operation.
type RestoreRequestInternal struct {
	RestoreRequest
	File *string
	Dir  *string
}

// RestoreTimestampRequest represents a restore by timestamp operation request.
type RestoreTimestampRequest struct {
	DestinationCuster *AerospikeCluster `json:"destination,omitempty"`
	Policy            *RestorePolicy    `json:"policy,omitempty"`
	Time              int64             `json:"time,omitempty" format:"int64"`
	Routine           string            `json:"routine,omitempty"`
}

// String satisfies the fmt.Stringer interface.
func (r RestoreRequest) String() string {
	request, err := json.Marshal(r)
	if err != nil {
		return err.Error()
	}
	return string(request)
}

// String satisfies the fmt.Stringer interface.
func (r RestoreTimestampRequest) String() string {
	request, err := json.Marshal(r)
	if err != nil {
		return err.Error()
	}
	return string(request)
}

// Validate validates the restore operation request.
func (r *RestoreRequest) Validate() error {
	if err := r.DestinationCuster.Validate(); err != nil {
		return err
	}
	if err := r.Policy.Validate(); err != nil {
		return err
	}
	if r.DestinationCuster == nil {
		return errors.New("destination cluster is not specified")
	}
	if err := r.SourceStorage.Validate(); err != nil {
		return err
	}
	if r.Policy == nil {
		return errors.New("restore policy is not specified")
	}
	if err := r.Policy.Validate(); err != nil {
		return err
	}
	return nil
}

// Validate validates the restore operation request.
func (r *RestoreTimestampRequest) Validate() error {
	if err := r.DestinationCuster.Validate(); err != nil {
		return err
	}
	if r.Policy == nil {
		return errors.New("restore policy is not specified")
	}
	if err := r.Policy.Validate(); err != nil {
		return err
	}
	if r.Time <= 0 {
		return errors.New("restore point in time should be positive")
	}
	if r.Routine == "" {
		return errors.New("routine to restore is not specified")
	}
	return nil
}
