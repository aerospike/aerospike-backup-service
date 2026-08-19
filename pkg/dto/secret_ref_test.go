package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/stretchr/testify/require"
)

func TestSecret_Validate_NilAgentWithPrefix(t *testing.T) {
	err := secret("secrets:asbackup:psw").Validate(false)
	require.Error(t, err)
	require.ErrorIs(t, err, decoder.ErrSecretValidation)
	require.ErrorContains(t, err, "secrets:asbackup:psw")
	require.ErrorContains(t, err, "secret agent")
}

func TestSecret_Validate_ValidReference(t *testing.T) {
	err := secret("secrets:resource:key").Validate(true)
	require.NoError(t, err)
}

func TestSecret_Validate_PlainValue(t *testing.T) {
	err := secret("plain-password").Validate(false)
	require.NoError(t, err)
}

func TestSecret_Validate_MalformedReference(t *testing.T) {
	err := secret("secrets:foo").Validate(true)
	require.Error(t, err)
	require.ErrorIs(t, err, decoder.ErrSecretValidation)
	require.ErrorContains(t, err, "secrets:foo")
	require.ErrorContains(t, err, "secrets:<resource>:<key>")
}

func TestCredentialsValidate_MalformedSecretRef(t *testing.T) {
	cluster := &AerospikeCluster{
		SeedNodes: []SeedNode{{HostName: "localhost", Port: 3000}},
		Credentials: &Credentials{
			User:     "user",
			Password: "secrets:foo",
		},
	}

	err := cluster.Validate(ValidationDefault)
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

	err := cluster.Validate(ValidationDefault)
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

	err := storage.Validate(ValidationDefault)
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

	err := storage.Validate(ValidationDefault)
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

	_, err := config.ToModel(ValidationSkipTLSFiles)
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

	err := request.Validate(ValidationDefault)
	require.Error(t, err)
	require.ErrorIs(t, err, errValidation)
	require.ErrorContains(t, err, "secret agent")
}
