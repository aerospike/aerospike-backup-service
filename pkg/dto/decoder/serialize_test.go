package decoder

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshal_Redact(t *testing.T) {
	cred := testCredentials{
		User:     "testUser",
		Password: literalPassword,
	}

	t.Run("yaml without redact", func(t *testing.T) {
		data, err := Marshal(&cred, YAML, false)
		require.NoError(t, err)
		assert.Contains(t, string(data), literalPassword)
	})

	t.Run("yaml with redact", func(t *testing.T) {
		data, err := Marshal(&cred, YAML, true)
		require.NoError(t, err)
		assert.Contains(t, string(data), "password")
		assert.NotContains(t, string(data), literalPassword)
	})

	t.Run("json with redact", func(t *testing.T) {
		data, err := Marshal(&cred, JSON, true)
		require.NoError(t, err)
		assert.Contains(t, string(data), "password")
		assert.NotContains(t, string(data), literalPassword)
	})
}
