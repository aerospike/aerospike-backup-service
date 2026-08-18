package decoder_test

import (
	"bytes"
	"fmt"
	"log/slog"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecret_String(t *testing.T) {
	assert.Empty(t, decoder.Secret("").String())
	assert.Equal(t, decoder.RedactedSecret, decoder.Secret("superSecretPassword").String())
	assert.Equal(t, "secrets:resource:key", decoder.Secret("secrets:resource:key").String())
}

func TestSecret_GoString(t *testing.T) {
	assert.Equal(t, `decoder.Secret("")`, decoder.Secret("").GoString())
	assert.Equal(t, `decoder.Secret("[secret]")`, decoder.Secret("superSecretPassword").GoString())
	assert.Equal(t, `decoder.Secret("secrets:resource:key")`, decoder.Secret("secrets:resource:key").GoString())

	output := fmt.Sprintf("%#v", decoder.Secret("superSecretPassword"))
	assert.NotContains(t, output, "superSecretPassword")
	assert.Contains(t, output, "[secret]")

	refOutput := fmt.Sprintf("%#v", decoder.Secret("secrets:resource:key"))
	assert.Contains(t, refOutput, "secrets:resource:key")
}

func TestSecret_LogValue(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	logger.Info("credentials", slog.Any("password", decoder.Secret("superSecretPassword")))

	assert.Contains(t, buf.String(), `"password":"`+decoder.RedactedSecret+`"`)
	assert.NotContains(t, buf.String(), "superSecretPassword")

	buf.Reset()
	logger.Info("credentials", slog.Any("password", decoder.Secret("secrets:resource:key")))
	assert.Contains(t, buf.String(), `"password":"secrets:resource:key"`)
}

func TestSecret_StringInCredentials(t *testing.T) {
	creds := &dto.Credentials{
		User:     "testUser",
		Password: "superSecretPassword",
	}

	output := fmt.Sprintf("user: %s, password: %s", creds.User, creds.Password)
	assert.NotContains(t, output, "superSecretPassword")
	assert.Contains(t, output, decoder.RedactedSecret)
}

func TestSecret_UnderlyingValueUnchanged(t *testing.T) {
	secret := decoder.Secret("superSecretPassword")
	require.Equal(t, "superSecretPassword", string(secret))

	ref := decoder.Secret("secrets:resource:key")
	require.Equal(t, "secrets:resource:key", string(ref))
}

func TestSecret_IsRef(t *testing.T) {
	assert.True(t, decoder.Secret("secrets:resource:key").IsRef())
	assert.False(t, decoder.Secret("secrets:foo").IsRef())
	assert.False(t, decoder.Secret("plain-password").IsRef())
	assert.False(t, decoder.Secret("").IsRef())
}

func TestSecret_HasRefPrefix(t *testing.T) {
	assert.True(t, decoder.Secret("secrets:resource:key").HasRefPrefix())
	assert.True(t, decoder.Secret("secrets:foo").HasRefPrefix())
	assert.False(t, decoder.Secret("plain-password").HasRefPrefix())
}

func TestSecret_DisplayString_MalformedRef(t *testing.T) {
	assert.Equal(t, decoder.RedactedSecret, decoder.Secret("secrets:foo").DisplayString())
}
