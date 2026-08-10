package dto

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestSecretString_StringRedacts(t *testing.T) {
	t.Parallel()

	assert.Empty(t, SecretString{}.String())
	assert.Equal(t, secretRedactedPlaceholder, NewSecretString("super-secret").String())
}

func TestSecretString_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	type payload struct {
		Password *SecretString `json:"password,omitempty"`
	}

	original := payload{Password: SecretStringPtr("plain-password")}
	data, err := json.Marshal(original)
	require.NoError(t, err)
	assert.JSONEq(t, `{"password":"plain-password"}`, string(data))

	var decoded payload
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, "plain-password", SecretStringValue(decoded.Password))
}

func TestSecretString_JSONMarshalIsForWireFormatNotLogging(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(SecretStringPtr("plain-password"))
	require.NoError(t, err)
	assert.Equal(t, `"plain-password"`, string(data))
}

func TestSecretString_YAMLRoundTrip(t *testing.T) {
	t.Parallel()

	type payload struct {
		Password *SecretString `yaml:"password,omitempty"`
	}

	original := payload{Password: SecretStringPtr("plain-password")}
	data, err := yaml.Marshal(original)
	require.NoError(t, err)
	assert.Contains(t, string(data), "plain-password")

	var decoded payload
	require.NoError(t, yaml.Unmarshal(data, &decoded))
	assert.Equal(t, "plain-password", SecretStringValue(decoded.Password))
}

func TestSecretString_DoesNotLeakInFormattedOutput(t *testing.T) {
	t.Parallel()

	secret := NewSecretString("do-not-log-me")
	assert.NotContains(t, secret.String(), "do-not-log-me")
	assert.NotContains(t, fmt.Sprintf("%v", secret), "do-not-log-me")
}
