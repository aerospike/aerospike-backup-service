package decoder_test

import (
	"bytes"
	"log/slog"
	"testing"

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
