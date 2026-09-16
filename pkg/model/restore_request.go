package model

import (
	"time"
)

// RestoreJobID represents the restore operation job id.
type RestoreJobID int64

// RestoreRequest represents a restore operation request.
type RestoreRequest struct {
	// The details of the Aerospike destination cluster.
	DestinationCluster AerospikeCluster
	// Restore policy to use in the operation.
	Policy RestorePolicy
	// Source storage configuration where backup data is located. Not nil.
	SourceStorage Storage
	// Secret Agent configuration (optional).
	SecretAgent *SecretAgent
	// Path to backup data in source storage.
	BackupDataPath string
}

// RestoreTimestampRequest represents a restore by timestamp operation request.
type RestoreTimestampRequest struct {
	// The details of the Aerospike destination cluster.
	DestinationCluster AerospikeCluster
	// Restore policy to use in the operation.
	Policy RestorePolicy
	// Secret Agent configuration (optional).
	SecretAgent *SecretAgent
	// Required epoch time for recovery. The closest backup before the timestamp will be applied.
	Time time.Time
	// The backup routine name used for backup path construction and chain discovery.
	RoutineName string
	// Source storage configuration where backup data is located. Not nil.
	Storage Storage
	// Disable reverse order of incremental backups optimization.
	DisableReordering bool
}

// NewRestoreRequest creates a new RestoreRequest.
func NewRestoreRequest(
	destinationCluster AerospikeCluster,
	policy RestorePolicy,
	sourceStorage Storage,
	secretAgent *SecretAgent,
	backupDataPath string,
) *RestoreRequest {
	return &RestoreRequest{
		DestinationCluster: destinationCluster,
		Policy:             policy,
		SourceStorage:      sourceStorage,
		SecretAgent:        secretAgent,
		BackupDataPath:     backupDataPath,
	}
}
