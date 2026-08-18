package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/require"
)

func TestValidateSecretRef_NilAgentWithPrefix(t *testing.T) {
	err := validateSecretRef("secrets:asbackup:psw", nil)
	require.Error(t, err)
	require.ErrorIs(t, err, errValidation)
	require.ErrorContains(t, err, "secrets:asbackup:psw")
	require.ErrorContains(t, err, "secret agent")
}

func TestValidateSecretRef_ValidReference(t *testing.T) {
	agent := &model.SecretAgent{ConnectionType: "tcp", Address: "localhost"}

	err := validateSecretRef("secrets:resource:key", agent)
	require.NoError(t, err)
}

func TestValidateSecretRef_PlainValue(t *testing.T) {
	err := validateSecretRef("plain-password", nil)
	require.NoError(t, err)
}

func TestCredentialsToModel_SecretRefWithoutAgent(t *testing.T) {
	cluster := &AerospikeCluster{
		SeedNodes: []SeedNode{{HostName: "localhost", Port: 3000}},
		Credentials: &Credentials{
			User:     "user",
			Password: "secrets:asbackup:psw",
		},
	}

	_, err := cluster.ToModel(model.NewConfig())
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

	_, err := storage.ToModel(model.NewConfig())
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

	_, err := storage.ToModel(model.NewConfig())
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
	config := model.NewConfig()
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

	_, err := request.ToModel(config)
	require.Error(t, err)
	require.ErrorIs(t, err, errValidation)
	require.ErrorContains(t, err, "secret agent")
}
