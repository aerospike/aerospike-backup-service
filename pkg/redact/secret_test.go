package redact

import (
	"bytes"
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecret_String(t *testing.T) {
	assert.Empty(t, Secret("").String())
	assert.Equal(t, Placeholder, Secret("superSecretPassword").String())
	assert.Equal(t, "secrets:resource:key", Secret("secrets:resource:key").String())
}

func TestSecret_GoString(t *testing.T) {
	assert.Equal(t, `redact.Secret("")`, Secret("").GoString())
	assert.Equal(t, `redact.Secret("[secret]")`, Secret("superSecretPassword").GoString())
	assert.Equal(t, `redact.Secret("secrets:resource:key")`, Secret("secrets:resource:key").GoString())

	output := fmt.Sprintf("%#v", Secret("superSecretPassword"))
	assert.NotContains(t, output, "superSecretPassword")
	assert.Contains(t, output, Placeholder)

	refOutput := fmt.Sprintf("%#v", Secret("secrets:resource:key"))
	assert.Contains(t, refOutput, "secrets:resource:key")
}

func TestSecret_LogValue(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	logger.Info("credentials", slog.Any("password", Secret("superSecretPassword")))

	assert.Contains(t, buf.String(), `"password":"`+Placeholder+`"`)
	assert.NotContains(t, buf.String(), "superSecretPassword")

	buf.Reset()
	logger.Info("credentials", slog.Any("password", Secret("secrets:resource:key")))
	assert.Contains(t, buf.String(), `"password":"secrets:resource:key"`)
}

func TestSecret_StringInCredentials(t *testing.T) {
	creds := struct {
		User     string
		Password Secret
	}{User: "testUser", Password: "superSecretPassword"}
	output := fmt.Sprintf("user: %s, password: %s", creds.User, creds.Password)
	assert.Contains(t, output, "testUser")
	assert.Contains(t, output, Placeholder)
	assert.NotContains(t, output, "superSecretPassword")
}

func TestSecret_UnderlyingValueUnchanged(t *testing.T) {
	secret := Secret("superSecretPassword")
	require.Equal(t, "superSecretPassword", secret.Reveal())

	ref := Secret("secrets:resource:key")
	require.Equal(t, "secrets:resource:key", string(ref))
}

func TestSecret_Hash(t *testing.T) {
	first := Secret("password").Hash()
	assert.Equal(t, first, Secret("password").Hash())
	assert.NotEqual(t, first, Secret("other-password").Hash(),
		"literal secrets must hash by value, not by the redaction placeholder")
	assert.NotEqual(t, Secret("").Hash(), Secret("password").Hash())
}

func TestSecret_IsRef(t *testing.T) {
	assert.True(t, Secret("secrets:resource:key").IsRef())
	assert.False(t, Secret("secrets:foo").IsRef())
	assert.False(t, Secret("plain-password").IsRef())
	assert.False(t, Secret("").IsRef())
}

func TestSecret_DisplayString_MalformedRef(t *testing.T) {
	assert.Equal(t, Placeholder, Secret("secrets:foo").DisplayString())
}
