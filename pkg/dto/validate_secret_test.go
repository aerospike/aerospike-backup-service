package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateSecret_ReferenceWithoutAgent(t *testing.T) {
	err := validateSecret("password", "secrets:asbackup:psw", false)
	require.Error(t, err)
	require.ErrorIs(t, err, errValidation)
	require.ErrorContains(t, err, "password")
	require.ErrorContains(t, err, "secrets:asbackup:psw")
	require.ErrorContains(t, err, "secret agent")
}

func TestValidateSecret_ValidReference(t *testing.T) {
	require.NoError(t, validateSecret("password", "secrets:resource:key", true))
}

func TestValidateSecret_PlainValue(t *testing.T) {
	require.NoError(t, validateSecret("password", "plain-password", false))
}

func TestValidateSecret_Empty(t *testing.T) {
	require.NoError(t, validateSecret("password", "", false))
}

func TestValidateSecret_MalformedReference(t *testing.T) {
	err := validateSecret("key-secret", "secrets:foo", true)
	require.Error(t, err)
	require.ErrorIs(t, err, errValidation)
	require.ErrorContains(t, err, "key-secret")
	require.ErrorContains(t, err, "secrets:<resource>:<key>")
}

// A literal password that happens to start with "secrets:" is malformed rather than a
// reference, so it reaches the same branch. The message must not carry it into the API
// error body or the log.
func TestValidateSecret_MalformedReferenceKeepsTheValueOut(t *testing.T) {
	err := validateSecret("password", "secrets:hunter2", false)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "hunter2")
	require.NotContains(t, err.Error(), "secrets:hunter2")
}

func TestCredentialsValidate_MalformedSecretRef(t *testing.T) {
	cluster := &AerospikeCluster{
		SeedNodes: []SeedNode{{HostName: "localhost", Port: 3000}},
		Credentials: &Credentials{
			User:     "user",
			Password: "secrets:foo",
		},
	}

	err := cluster.Validate()
	require.Error(t, err)
	require.ErrorIs(t, err, errValidation)
	require.ErrorContains(t, err, "password")
	require.ErrorContains(t, err, "secrets:<resource>:<key>")
}

func TestCredentialsToModel_SecretRefWithoutAgent(t *testing.T) {
	cluster := &AerospikeCluster{
		SeedNodes: []SeedNode{{HostName: "localhost", Port: 3000}},
		Credentials: &Credentials{
			User:     "user",
			Password: "secrets:asbackup:psw",
		},
	}

	err := cluster.Validate()
	require.Error(t, err)
	require.ErrorIs(t, err, errValidation)
	require.ErrorContains(t, err, "password")
	require.ErrorContains(t, err, "secret agent")
}

func TestS3StorageToModel_SecretRefWithoutAgent(t *testing.T) {
	storage := &Storage{
		S3Storage: &S3Storage{
			Bucket:          "bucket",
			S3Region:        "us-east-1",
			AccessKeyID:     "secrets:resource:key-id",
			SecretAccessKey: "plain-key",
		},
	}

	err := storage.Validate()
	require.Error(t, err)
	require.ErrorIs(t, err, errValidation)
	require.ErrorContains(t, err, "access-key-id")
	require.ErrorContains(t, err, "secret agent")
}

func TestGcpStorageToModel_SecretRefWithoutAgent(t *testing.T) {
	storage := &Storage{
		GcpStorage: &GcpStorage{
			BucketName: "bucket",
			Key:        "secrets:resource:key-json",
		},
	}

	err := storage.Validate()
	require.Error(t, err)
	require.ErrorIs(t, err, errValidation)
	require.ErrorContains(t, err, "key-json")
	require.ErrorContains(t, err, "secret agent")
}

func TestConfigToModel_SecretRefWithoutAgent(t *testing.T) {
	config := &Config{
		AerospikeClusters: map[string]*AerospikeCluster{
			"cluster1": {
				SeedNodes: []SeedNode{{HostName: "localhost", Port: 3000}},
				Credentials: &Credentials{
					User:     "user",
					Password: "secrets:asbackup:psw",
				},
			},
		},
	}

	err := config.Validate()
	require.Error(t, err)
	require.ErrorIs(t, err, errValidation)
	require.ErrorContains(t, err, "secret agent")
}

func TestRestoreRequestToModel_SecretRefWithoutAgent(t *testing.T) {
	request := &RestoreRequest{
		DestinationClusterConfig: DestinationClusterConfig{
			Cluster: &AerospikeCluster{
				SeedNodes: []SeedNode{{HostName: "localhost", Port: 3000}},
				Credentials: &Credentials{
					User:     "user",
					Password: "secrets:asbackup:psw",
				},
			},
		},
		StorageConfig: StorageConfig{
			Storage: &Storage{
				LocalStorage: &LocalStorage{Path: "/tmp/backups"},
			},
		},
		BackupDataPath: "backup-path",
	}

	err := request.Validate()
	require.Error(t, err)
	require.ErrorIs(t, err, errValidation)
	require.ErrorContains(t, err, "secret agent")
}

func TestRestoreRequest_Validate_PolicySecretRefWithoutAgent(t *testing.T) {
	request := &RestoreRequest{
		DestinationClusterConfig: DestinationClusterConfig{
			Cluster: &AerospikeCluster{
				SeedNodes: []SeedNode{{HostName: "localhost", Port: 3000}},
			},
		},
		StorageConfig: StorageConfig{
			Storage: &Storage{
				LocalStorage: &LocalStorage{Path: "/tmp/backups"},
			},
		},
		BackupDataPath: "backup-path",
		Policy: &RestorePolicy{
			BaseRestorePolicy: BaseRestorePolicy{
				EncryptionPolicy: &EncryptionPolicy{
					Mode:      EncryptionModeAES256,
					KeySecret: "secrets:resource:key",
				},
			},
		},
	}

	err := request.Validate()
	require.Error(t, err)
	require.ErrorIs(t, err, errValidation)
	require.ErrorContains(t, err, "key-secret")
	require.ErrorContains(t, err, "secret agent")
}

func TestRestoreTimestampRequest_Validate_PolicySecretRefWithoutInlineAgent(t *testing.T) {
	request := &RestoreTimestampRequest{
		Time:    1_700_000_000_000,
		Routine: "daily",
		Policy: &TimestampRestorePolicy{
			BaseRestorePolicy: BaseRestorePolicy{
				EncryptionPolicy: &EncryptionPolicy{
					Mode:      EncryptionModeAES256,
					KeySecret: "secrets:resource:key",
				},
			},
		},
	}

	err := request.Validate()
	require.NoError(t, err)
}
