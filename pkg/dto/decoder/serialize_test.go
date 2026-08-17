package decoder_test

import (
	"bytes"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshal_Redact(t *testing.T) {
	config := dto.Config{
		AerospikeClusters: map[string]*dto.AerospikeCluster{
			"cluster1": {
				Credentials: &dto.Credentials{
					User:     "testUser",
					Password: "superSecretPassword",
				},
			},
		},
	}

	t.Run("yaml without redact", func(t *testing.T) {
		data, err := decoder.Marshal(&config, decoder.YAML, false)
		require.NoError(t, err)
		assert.Contains(t, string(data), "superSecretPassword")
	})

	t.Run("yaml with redact", func(t *testing.T) {
		data, err := decoder.Marshal(&config, decoder.YAML, true)
		require.NoError(t, err)
		assert.Contains(t, string(data), decoder.RedactedSecret)
		assert.NotContains(t, string(data), "superSecretPassword")
	})
}

func TestSerialize_Redact(t *testing.T) {
	config := dto.Config{
		AerospikeClusters: map[string]*dto.AerospikeCluster{
			"cluster1": {
				Credentials: &dto.Credentials{
					Password: "superSecretPassword",
				},
			},
		},
	}

	var buf bytes.Buffer
	err := decoder.Serialize(&buf, &config, decoder.JSON, true)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), decoder.RedactedSecret)
	assert.NotContains(t, buf.String(), "superSecretPassword")
}
