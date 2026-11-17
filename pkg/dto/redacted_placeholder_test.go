package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/redact"
	"github.com/stretchr/testify/require"
)

// Restore requests carry their own clusters and storage and are never merged with stored
// secrets, so the placeholder is as meaningless there as in a configuration file.
func TestRestoreRequests_Validate_RejectRedactedPlaceholder(t *testing.T) {
	cluster := &AerospikeCluster{
		SeedNodes:   []SeedNode{{HostName: "host", Port: 3000}},
		Credentials: &Credentials{User: "admin", Password: redact.Placeholder},
	}

	restore := RestoreRequest{
		DestinationClusterConfig: DestinationClusterConfig{Cluster: cluster},
		StorageConfig:            StorageConfig{Storage: &Storage{LocalStorage: &LocalStorage{Path: "p"}}},
		Policy:                   &RestorePolicy{},
		BackupDataPath:           "test/path",
	}
	err := restore.Validate()
	require.ErrorIs(t, err, errValidation)
	require.ErrorContains(t, err, "destination.credentials.password: ")

	timestamp := RestoreTimestampRequest{
		DestinationClusterConfig: DestinationClusterConfig{Cluster: cluster},
		Policy:                   &TimestampRestorePolicy{},
		Time:                     1_739_538_000_000,
		Routine:                  "daily",
	}
	err = timestamp.Validate()
	require.ErrorIs(t, err, errValidation)
	require.ErrorContains(t, err, "destination.credentials.password: ")
}
