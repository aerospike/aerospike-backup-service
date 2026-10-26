package dto

import (
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/redact"
	"github.com/stretchr/testify/require"
)

// A configuration that still holds the redaction placeholder was never merged with stored
// secrets - it comes from a file, a URL or a reload - so the placeholder would be used as the
// credential itself.
func TestConfig_Validate_RejectsRedactedPlaceholder(t *testing.T) {
	config, err := NewFromReader[Config](strings.NewReader(`
aerospike-clusters:
  c1:
    seed-nodes: [{host-name: localhost, port: 3000}]
    credentials: {user: admin, password: "[secret]"}
backup-policies:
  p1:
    encryption: {mode: AES256, key-secret: "[secret]"}
`), decoder.YAML)
	require.NoError(t, err)

	err = config.Validate()

	require.ErrorIs(t, err, errValidation)
	require.ErrorContains(t, err, "aerospike-clusters.c1.credentials.password: ")
	require.ErrorContains(t, err, "redaction placeholder and has no stored value")
}

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
