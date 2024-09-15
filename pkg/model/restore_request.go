package model

import (
	"encoding/json"
	"time"
)

// RestoreJobID represents the restore operation job id.
type RestoreJobID int

// RestoreRequest represents a restore operation request.
// @Description RestoreRequest represents a restore operation request.
type RestoreRequest struct {
	DestinationCuster *AerospikeCluster
	Policy            *RestorePolicy
	SourceStorage     Storage
	SecretAgent       *SecretAgent
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
	DestinationCuster *AerospikeCluster
	// Restore policy to use in the operation.
	Policy *RestorePolicy
	// Secret Agent configuration (optional).
	SecretAgent *SecretAgent
	// Required epoch time for recovery. The closest backup before the timestamp will be applied.
	Time time.Time
	// The backup routine name.
	Routine string
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

// NewRestoreRequest creates a new RestoreRequest.
func NewRestoreRequest(
	destinationCluster *AerospikeCluster,
	policy *RestorePolicy,
	sourceStorage Storage,
	secretAgent *SecretAgent,
) *RestoreRequest {
	return &RestoreRequest{
		DestinationCuster: destinationCluster,
		Policy:            policy,
		SourceStorage:     sourceStorage,
		SecretAgent:       secretAgent,
	}
}
