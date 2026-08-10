package dto

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecretValue = "test-secret-value"

func TestCredentials_LogValue_RedactsSecrets(t *testing.T) {
	t.Parallel()

	creds := &Credentials{
		User:         ptr.Of("admin"),
		Password:     ptr.Of(testSecretValue),
		PasswordPath: ptr.Of("/etc/password.txt"),
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	logger.Info("credentials", slog.Any("credentials", creds))

	output := buf.String()
	assert.NotContains(t, output, testSecretValue)
	assert.NotContains(t, output, "/etc/password.txt")
	assert.Contains(t, output, logRedactedPlaceholder)
	assert.Contains(t, output, `"user":"admin"`)
}

func TestS3Storage_LogValue_RedactsAccessKeys(t *testing.T) {
	t.Parallel()

	storage := &S3Storage{
		Bucket:          "bucket",
		S3Region:        "us-east-1",
		AccessKeyID:     ptr.Of(testSecretValue),
		SecretAccessKey: ptr.Of(testSecretValue),
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	logger.Info("storage", slog.Any("s3", storage))

	output := buf.String()
	assert.NotContains(t, output, testSecretValue)
	assert.Contains(t, output, logRedactedPlaceholder)
	assert.Contains(t, output, `"bucket":"bucket"`)
}

func TestEncryptionPolicy_LogValue_RedactsKeyMaterial(t *testing.T) {
	t.Parallel()

	policy := &EncryptionPolicy{
		Mode:      EncryptAES256,
		KeyFile:   ptr.Of("/etc/encryption.key"),
		KeyEnv:    ptr.Of("BACKUP_KEY"),
		KeySecret: ptr.Of(testSecretValue),
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	logger.Info("policy", slog.Any("encryption", policy))

	output := buf.String()
	assert.NotContains(t, output, testSecretValue)
	assert.NotContains(t, output, "/etc/encryption.key")
	assert.Contains(t, output, logRedactedPlaceholder)
	assert.Contains(t, output, `"key-env":"BACKUP_KEY"`)
}

func TestRestoreRequest_LogValue_RedactsGcpKeyJSON(t *testing.T) {
	t.Parallel()

	const gcpKeyJSON = `{"private_key":"-----BEGIN PRIVATE KEY-----\nsecret\n-----END PRIVATE KEY-----"}`

	req := &RestoreRequest{
		DestinationClusterConfig: DestinationClusterConfig{
			Name: "dest-cluster",
		},
		StorageConfig: StorageConfig{
			Storage: &Storage{
				GcpStorage: &GcpStorage{
					BucketName: "bucket",
					Key:        gcpKeyJSON,
				},
			},
		},
		Policy:         &RestorePolicy{},
		BackupDataPath: "backup/path",
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	logger.Info("restore", slog.Any("request", req))

	output := buf.String()
	assert.NotContains(t, output, gcpKeyJSON)
	assert.NotContains(t, output, "BEGIN PRIVATE KEY")
	assert.Contains(t, output, logRedactedPlaceholder)
	assert.Contains(t, output, `"bucket-name":"bucket"`)
}

func TestConfig_LogValue_RedactsNestedSecrets(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		AerospikeClusters: map[string]*AerospikeCluster{
			"cluster1": {
				SeedNodes: []SeedNode{{HostName: "localhost", Port: Port(3000)}},
				Credentials: &Credentials{
					User:     ptr.Of("tester"),
					Password: ptr.Of(testSecretValue),
				},
			},
		},
		Storage: map[string]*Storage{
			"aws": {
				S3Storage: &S3Storage{
					Bucket:          "bucket",
					S3Region:        "us-east-1",
					AccessKeyID:     ptr.Of(testSecretValue),
					SecretAccessKey: ptr.Of(testSecretValue),
				},
			},
		},
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	logger.Info("config", slog.Any("config", cfg))

	output := buf.String()
	assert.NotContains(t, output, testSecretValue)
	assert.Contains(t, output, logRedactedPlaceholder)
	assert.Contains(t, output, `"user":"tester"`)
}

func TestTLS_LogValue_RedactsKeyfilePassword(t *testing.T) {
	t.Parallel()

	tlsConfig := &TLS{
		KeyfilePassword: ptr.Of(testSecretValue),
		ClientTLS: ClientTLS{
			Keyfile: ptr.Of("/etc/tls/key.pem"),
		},
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	logger.Info("tls", slog.Any("tls", tlsConfig))

	output := buf.String()
	require.NotContains(t, output, testSecretValue)
	require.NotContains(t, output, "/etc/tls/key.pem")
	assert.Contains(t, output, logRedactedPlaceholder)
}
