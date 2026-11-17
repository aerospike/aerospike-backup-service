package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/require"
)

func TestValidateSecretAgent_ReferenceWithoutAgent(t *testing.T) {
	err := validateSecretAgent("password", "secrets:asbackup:psw", false)
	require.ErrorIs(t, err, errValidation)
	require.ErrorContains(t, err, "password")
	require.ErrorContains(t, err, "secrets:asbackup:psw")
	require.ErrorContains(t, err, "secret agent")
}

func TestValidateSecretAgent_Accepts(t *testing.T) {
	require.NoError(t, validateSecretAgent("password", "secrets:resource:key", true))
	require.NoError(t, validateSecretAgent("password", "plain-password", false))
	require.NoError(t, validateSecretAgent("password", "", false))
}

func TestValidateSecretForm_Accepts(t *testing.T) {
	for _, secret := range []Secret{"secrets:resource:key", "plain-password", ""} {
		require.NoError(t, validateSecretForm("password", secret))
	}
}

func TestValidateSecretForm_MalformedReference(t *testing.T) {
	err := validateSecretForm("key-secret", "secrets:foo")
	require.ErrorIs(t, err, errValidation)
	require.ErrorContains(t, err, "key-secret")
	require.ErrorContains(t, err, "secrets:<resource>:<key>")
}

// A literal password that happens to start with "secrets:" is malformed rather than a
// reference, so it reaches the same branch. The message must not carry it into the API
// error body or the log.
func TestValidateSecretForm_MalformedReferenceKeepsTheValueOut(t *testing.T) {
	err := validateSecretForm("password", "secrets:hunter2")
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

	require.NoError(t, cluster.Validate())

	_, err := cluster.ToModel(model.NewConfig())
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

	require.NoError(t, storage.Validate())

	_, err := storage.ToModel(model.NewConfig())
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

	require.NoError(t, storage.Validate())

	_, err := storage.ToModel(model.NewConfig())
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

	require.NoError(t, config.Validate())

	_, err := config.ToModel()
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

	require.NoError(t, request.Validate())

	_, err := request.ToModel(model.NewConfig())
	require.ErrorIs(t, err, errValidation)
	require.ErrorContains(t, err, "secret agent")
}

// A restore policy's key-secret is resolved with the request's agent, so the reference is checked
// once that agent is resolved.
func TestRestoreRequest_ToModel_PolicySecretRef(t *testing.T) {
	request := func(agent *SecretAgentConfig) *RestoreRequest {
		return &RestoreRequest{
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
			SecretAgentConfig: agent,
			BackupDataPath:    "backup-path",
			Policy: &RestorePolicy{
				BaseRestorePolicy: BaseRestorePolicy{
					EncryptionPolicy: &EncryptionPolicy{
						Mode:      EncryptionModeAES256,
						KeySecret: "secrets:resource:key",
					},
				},
			},
		}
	}
	inline := &SecretAgentConfig{SecretAgent: &SecretAgent{
		ConnectionType: "tcp", Address: "secret-agent.aerospike.svc", Port: new(Port(3005)),
	}}

	_, err := request(inline).ToModel(model.NewConfig())
	require.NoError(t, err)

	_, err = request(nil).ToModel(model.NewConfig())
	require.ErrorIs(t, err, errValidation)
	require.ErrorContains(t, err, "key-secret")
	require.ErrorContains(t, err, "secret agent")
}

// A cluster's TLS key-file password is resolved with its credentials' agent (secret.ClusterTLSResolver),
// so the reference is checked once the credentials are resolved.
func TestAerospikeCluster_ToModel_TLSPasswordSecretRef(t *testing.T) {
	cluster := func(credentials *Credentials) *AerospikeCluster {
		return &AerospikeCluster{
			SeedNodes:   []SeedNode{{HostName: "localhost", Port: 3000, TLSName: "tls"}},
			Credentials: credentials,
			TLS: &TLS{
				ClientTLS:       ClientTLS{Certfile: "/certs/cert.pem", Keyfile: "/certs/key.pem"},
				KeyfilePassword: "secrets:resource:tls-pw",
			},
		}
	}
	withAgent := &Credentials{User: "user", Password: "pw", SecretAgentConfig: SecretAgentConfig{
		SecretAgent: &SecretAgent{ConnectionType: "tcp", Address: "secret-agent.aerospike.svc", Port: new(Port(3005))},
	}}

	_, err := cluster(withAgent).ToModel(model.NewConfig())
	require.NoError(t, err)

	_, err = cluster(nil).ToModel(model.NewConfig())
	require.ErrorIs(t, err, errValidation)
	require.ErrorContains(t, err, "key-file-password")
	require.ErrorContains(t, err, "secret agent")
}

// The agent of a timestamp restore may come from its routine, so Validate accepts the reference
// and it is checked once the routine is resolved.
func TestRestoreTimestampRequest_ToModel_PolicySecretRef(t *testing.T) {
	config := model.NewConfig()
	cluster := &model.AerospikeCluster{}
	storage := &model.LocalStorage{Path: "routine-path"}
	require.NoError(t, config.AddRoutine(&model.BackupRoutine{
		Name: "with-agent", SourceCluster: cluster, Storage: storage, SecretAgent: &model.SecretAgent{},
	}))
	require.NoError(t, config.AddRoutine(&model.BackupRoutine{
		Name: "without-agent", SourceCluster: cluster, Storage: storage,
	}))
	request := func(routine string) *RestoreTimestampRequest {
		return &RestoreTimestampRequest{
			Time:    1_700_000_000_000,
			Routine: routine,
			Policy: &TimestampRestorePolicy{
				BaseRestorePolicy: BaseRestorePolicy{
					EncryptionPolicy: &EncryptionPolicy{
						Mode:      EncryptionModeAES256,
						KeySecret: "secrets:resource:key",
					},
				},
			},
		}
	}

	require.NoError(t, request("without-agent").Validate())

	_, err := request("with-agent").ToModel(config)
	require.NoError(t, err)

	_, err = request("without-agent").ToModel(config)
	require.ErrorIs(t, err, errValidation)
	require.ErrorContains(t, err, "key-secret")
	require.ErrorContains(t, err, "secret agent")
}
