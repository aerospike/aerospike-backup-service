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
	// Namespace is the namespace the backup was taken from. A restore by timestamp sets it on
	// each backup of a namespace's chain; a restore by path leaves it empty.
	Namespace string
	// DisableReordering keeps a chain of backups oldest first under the unique (CREATE_ONLY)
	// policy too, which otherwise applies it newest first, and keeps a chain into an empty
	// namespace from switching to that policy.
	DisableReordering bool
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
