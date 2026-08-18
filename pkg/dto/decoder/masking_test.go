package decoder_test

import (
	"bytes"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPasswordMasking(t *testing.T) {
	creds := &dto.Credentials{
		User:     "testUser",
		Password: "superSecretPassword",
	}

	t.Run("without redaction", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, nil))
		logger.Info("cluster credentials", slog.Any("credentials", creds))

		assert.Contains(t, buf.String(), "superSecretPassword")
	})

	t.Run("with redaction", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
			ReplaceAttr: decoder.RedactSecretsReplaceAttr(),
		}))

		logger.Info("cluster credentials", slog.Any("credentials", dto.Config{
			AerospikeClusters: map[string]*dto.AerospikeCluster{
				"cluster1": {
					Credentials: creds,
				},
			},
		}))

		output := buf.String()
		assert.Contains(t, output, `"user":"testUser"`)
		assert.Contains(t, output, `"password":"`+decoder.RedactedSecret+`"`)
		assert.NotContains(t, output, "superSecretPassword")
	})

	t.Run("redact for JSON response", func(t *testing.T) {
		config := dto.Config{
			AerospikeClusters: map[string]*dto.AerospikeCluster{
				"cluster1": {
					Credentials: creds,
				},
			},
		}

		data, err := decoder.Marshal(&config, decoder.JSON, true)
		require.NoError(t, err)

		output := string(data)
		assert.Contains(t, output, `"user":"testUser"`)
		assert.Contains(t, output, `"password":"`+decoder.RedactedSecret+`"`)
		assert.NotContains(t, output, "superSecretPassword")
	})

	t.Run("marshal without redact preserves secrets", func(t *testing.T) {
		config := dto.Config{
			AerospikeClusters: map[string]*dto.AerospikeCluster{
				"cluster1": {
					Credentials: creds,
				},
			},
		}

		data, err := decoder.Marshal(&config, decoder.JSON, false)
		require.NoError(t, err)

		output := string(data)
		assert.Contains(t, output, "superSecretPassword")
	})
}

func TestStringRedactSecrets(t *testing.T) {
	creds := &dto.Credentials{
		User:     "testUser",
		Password: "superSecretPassword",
	}

	output := fmt.Sprintf("user: %s, password: %s", creds.User, creds.Password)
	assert.NotContains(t, output, "superSecretPassword")
}

func TestRedactSecrets_PreservesTime(t *testing.T) {
	created := time.UnixMilli(1000).UTC()
	finished := time.UnixMilli(5000).UTC()

	original := map[string][]dto.BackupDetails{
		"routine1": {
			{
				Key:       "backup1",
				Created:   created,
				Timestamp: 1000,
				Finished:  finished,
			},
		},
	}

	redacted := decoder.RedactSecrets(original).(map[string][]dto.BackupDetails)
	require.Len(t, redacted["routine1"], 1)
	assert.Equal(t, created, redacted["routine1"][0].Created)
	assert.Equal(t, finished, redacted["routine1"][0].Finished)
	assert.Equal(t, int64(1000), redacted["routine1"][0].Timestamp)
}
