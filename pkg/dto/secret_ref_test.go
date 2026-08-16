package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/require"
)

func testSecretAgentConfig() SecretAgentConfig {
	return SecretAgentConfig{SecretAgentName: "sa"}
}

func testConfigWithSecretAgent(t *testing.T) *model.Config {
	t.Helper()

	config := model.NewConfig()
	require.NoError(t, config.AddSecretAgent("sa", &model.SecretAgent{
		ConnectionType: "tcp",
		Address:        "localhost",
		Port:           ptr.Of(model.Port(8080)),
	}))

	return config
}

func TestValidateSecretRef_NilAgentWithPrefix(t *testing.T) {
	err := validateSecretRef("secrets:asbackup:psw", nil)
	require.Error(t, err)
	require.ErrorIs(t, err, errValidation)
	require.ErrorContains(t, err, "secrets:asbackup:psw")
	require.ErrorContains(t, err, "secret agent")
}

func TestValidateSecretRef_MalformedReference(t *testing.T) {
	agent := &model.SecretAgent{ConnectionType: "tcp", Address: "localhost"}

	err := validateSecretRef("secrets:foo", agent)
	require.Error(t, err)
	require.ErrorIs(t, err, errValidation)
	require.ErrorContains(t, err, "secrets:foo")
	require.ErrorContains(t, err, "secrets:<resource>:<secret>")
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
			User:     ptr.Of("user"),
			Password: ptr.Of("secrets:asbackup:psw"),
		},
	}

	_, err := cluster.ToModel(model.NewConfig())
	require.Error(t, err)
	require.ErrorIs(t, err, errValidation)
	require.ErrorContains(t, err, "password")
	require.ErrorContains(t, err, "secret agent")
}

func TestCredentialsToModel_MalformedSecretRef(t *testing.T) {
	cluster := &AerospikeCluster{
		SeedNodes: []SeedNode{{HostName: "localhost", Port: 3000}},
		Credentials: &Credentials{
			User:              ptr.Of("user"),
			Password:          ptr.Of("secrets:foo"),
			SecretAgentConfig: testSecretAgentConfig(),
		},
	}

	_, err := cluster.ToModel(testConfigWithSecretAgent(t))
	require.Error(t, err)
	require.ErrorIs(t, err, errValidation)
	require.ErrorContains(t, err, "password")
	require.ErrorContains(t, err, "secrets:<resource>:<secret>")
}

func TestS3StorageToModel_SecretRefWithoutAgent(t *testing.T) {
	storage := &Storage{
		S3Storage: &S3Storage{
			Bucket:          "bucket",
			S3Region:        "us-east-1",
			AccessKeyID:     ptr.Of("secrets:resource:key-id"),
			SecretAccessKey: ptr.Of("plain-key"),
		},
	}

	_, err := storage.ToModel(model.NewConfig())
	require.Error(t, err)
	require.ErrorIs(t, err, errValidation)
	require.ErrorContains(t, err, "access-key-id")
	require.ErrorContains(t, err, "secret agent")
}

func TestAzureStorageToModel_MalformedSecretRef(t *testing.T) {
	storage := &Storage{
		AzureStorage: &AzureStorage{
			Endpoint:          "https://account.blob.core.windows.net",
			ContainerName:     "container",
			AccountName:       "account",
			AccountKey:        "secrets:bad",
			SecretAgentConfig: testSecretAgentConfig(),
		},
	}

	_, err := storage.ToModel(testConfigWithSecretAgent(t))
	require.Error(t, err)
	require.ErrorIs(t, err, errValidation)
	require.ErrorContains(t, err, "account-key")
	require.ErrorContains(t, err, "secrets:<resource>:<secret>")
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
					User:     ptr.Of("user"),
					Password: ptr.Of("secrets:asbackup:psw"),
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
					User:     ptr.Of("user"),
					Password: ptr.Of("secrets:asbackup:psw"),
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
