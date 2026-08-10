package log

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const secretSentinel = "s3nt1nel-do-not-log-2f9c"

func newRedactingLogger(buf *bytes.Buffer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: RedactSecretsAttr,
	}))
}

func TestRedactSecretsAttr_RedactsNestedSecretString(t *testing.T) {
	t.Parallel()

	config := &dto.Config{
		AerospikeClusters: map[string]*dto.AerospikeCluster{
			"cluster1": {
				SeedNodes: []dto.SeedNode{{HostName: "localhost", Port: 3000}},
				Credentials: &dto.Credentials{
					User:     ptr.Of("admin"),
					Password: dto.SecretStringPtr(secretSentinel),
				},
			},
		},
		Storage: map[string]*dto.Storage{
			"aws": {
				S3Storage: &dto.S3Storage{
					Bucket:          "b",
					AccessKeyID:     dto.SecretStringPtr(secretSentinel),
					SecretAccessKey: dto.SecretStringPtr(secretSentinel),
				},
			},
		},
	}

	var buf bytes.Buffer
	logger := newRedactingLogger(&buf, slog.LevelDebug)
	logger.Debug("config", slog.Any("config", *config))

	output := buf.String()
	assert.NotContains(t, output, secretSentinel, "secret leaked into log output")
	assert.Contains(t, output, redactedPlaceholder)
	// Non-secret values must survive.
	assert.Contains(t, output, "localhost")
	assert.Contains(t, output, "admin")
}

func TestRedactSecretsAttr_RestoreRequestSecrets(t *testing.T) {
	t.Parallel()

	req := dto.RestoreRequest{
		StorageConfig: dto.StorageConfig{
			Storage: &dto.Storage{
				S3Storage: &dto.S3Storage{
					Bucket:          "b",
					SecretAccessKey: dto.SecretStringPtr(secretSentinel),
				},
			},
		},
	}

	var buf bytes.Buffer
	logger := newRedactingLogger(&buf, slog.LevelInfo)
	logger.Info("New restore job", slog.Any("request", req))

	output := buf.String()
	assert.NotContains(t, output, secretSentinel)
	assert.Contains(t, output, redactedPlaceholder)
}

func TestRedactSecretsAttr_PreservesNonSecretStrings(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := newRedactingLogger(&buf, slog.LevelInfo)
	// A password-shaped literal that is NOT a SecretString must NOT be redacted.
	logger.Info("plain", slog.String("note", "the word backup is fine here"))

	output := buf.String()
	assert.Contains(t, output, "the word backup is fine here")
	assert.NotContains(t, output, redactedPlaceholder)
}

func TestRedactSecretsAttr_HandlesNilAndEmpty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := newRedactingLogger(&buf, slog.LevelInfo)

	cluster := dto.AerospikeCluster{
		Credentials: &dto.Credentials{
			User:     ptr.Of("admin"),
			Password: nil,
		},
	}
	require.NotPanics(t, func() {
		logger.Info("cluster", slog.Any("cluster", cluster))
	})

	output := buf.String()
	assert.NotContains(t, output, redactedPlaceholder)
	assert.Contains(t, output, "admin")
}

// TestRedactSecretsAttr_SentinelNeverLeaks is the regression guard: it walks a
// realistic config through the real handler and asserts the sentinel secret is
// absent from every byte of output.
func TestRedactSecretsAttr_SentinelNeverLeaks(t *testing.T) {
	t.Parallel()

	config := dto.Config{
		AerospikeClusters: map[string]*dto.AerospikeCluster{
			"c": {Credentials: &dto.Credentials{Password: dto.SecretStringPtr(secretSentinel)}},
		},
	}

	// Sanity: the sentinel really is present in the source data via wire format.
	raw, err := json.Marshal(config)
	require.NoError(t, err)
	require.Contains(t, string(raw), secretSentinel,
		"test setup broken: sentinel not present in marshaled config")

	var buf bytes.Buffer
	logger := newRedactingLogger(&buf, slog.LevelDebug)
	logger.Debug("service configuration", slog.Any("config", config))

	assert.NotContains(t, buf.String(), secretSentinel)
}

// TestRedactSecretsAttr_TextHandlerNeverLeaks guards the PLAIN log format,
// where slog renders values via fmt rather than JSON reflection.
func TestRedactSecretsAttr_TextHandlerNeverLeaks(t *testing.T) {
	t.Parallel()

	cluster := dto.AerospikeCluster{
		Credentials: &dto.Credentials{Password: dto.SecretStringPtr(secretSentinel)},
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level:       slog.LevelDebug,
		ReplaceAttr: RedactSecretsAttr,
	}))
	logger.Debug("cluster", slog.Any("cluster", cluster))

	assert.NotContains(t, buf.String(), secretSentinel)
}
