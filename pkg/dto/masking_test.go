package dto

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/m-mizutani/masq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// redactForJSON reuses masq's slog ReplaceAttr function to produce a redacted
// deep copy of v that is safe to pass to json.Marshal, without depending on a
// separate JSON-specific masking library.
func redactForJSON(tag string, v any) any {
	redact := masq.New(masq.WithTag(tag))
	return redact(nil, slog.Any("v", v)).Value.Any()
}

func TestPasswordMasking(t *testing.T) {
	creds := &Credentials{
		User:     "testUser",
		Password: "superSecretPassword",
	}

	t.Run("without masq", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, nil))
		logger.Info("cluster credentials", slog.Any("credentials", creds))

		assert.Contains(t, buf.String(), "superSecretPassword")
	})

	t.Run("with masq", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
			ReplaceAttr: masq.New(masq.WithTag("secret")),
		}))

		logger.Info("cluster credentials", slog.Any("credentials", Config{
			AerospikeClusters: map[string]*AerospikeCluster{
				"cluster1": {
					Credentials: creds,
				},
			},
		}))

		output := buf.String()
		assert.Contains(t, output, `"user":"testUser"`)
		assert.Contains(t, output, `"password":"`+masq.DefaultRedactMessage+`"`)
		assert.NotContains(t, output, "superSecretPassword")
	})

	t.Run("redact for JSON response", func(t *testing.T) {
		config := Config{
			AerospikeClusters: map[string]*AerospikeCluster{
				"cluster1": {
					Credentials: creds,
				},
			},
		}

		redacted := redactForJSON("secret", &config)

		data, err := json.Marshal(redacted)
		require.NoError(t, err)

		output := string(data)
		assert.Contains(t, output, `"user":"testUser"`)
		assert.Contains(t, output, `"password":"`+masq.DefaultRedactMessage+`"`)
		assert.NotContains(t, output, "superSecretPassword")
	})
}
