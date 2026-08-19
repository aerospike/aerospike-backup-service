package decoder

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
	assert.Equal(t, RedactedSecret, Secret("superSecretPassword").String())
	assert.Equal(t, "secrets:resource:key", Secret("secrets:resource:key").String())
}

func TestSecret_GoString(t *testing.T) {
	assert.Equal(t, `decoder.Secret("")`, Secret("").GoString())
	assert.Equal(t, `decoder.Secret("[secret]")`, Secret("superSecretPassword").GoString())
	assert.Equal(t, `decoder.Secret("secrets:resource:key")`, Secret("secrets:resource:key").GoString())

	output := fmt.Sprintf("%#v", Secret("superSecretPassword"))
	assert.NotContains(t, output, "superSecretPassword")
	assert.Contains(t, output, RedactedSecret)

	refOutput := fmt.Sprintf("%#v", Secret("secrets:resource:key"))
	assert.Contains(t, refOutput, "secrets:resource:key")
}

func TestSecret_LogValue(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	logger.Info("credentials", slog.Any("password", Secret("superSecretPassword")))

	assert.Contains(t, buf.String(), `"password":"`+RedactedSecret+`"`)
	assert.NotContains(t, buf.String(), "superSecretPassword")

	buf.Reset()
	logger.Info("credentials", slog.Any("password", Secret("secrets:resource:key")))
	assert.Contains(t, buf.String(), `"password":"secrets:resource:key"`)
}

func TestSecret_StringInCredentials(t *testing.T) {
	output := fmt.Sprintf("user: %s, password: %s", testCreds.User, testCreds.Password)
	assert.Contains(t, output, "testUser")
	assert.Contains(t, output, RedactedSecret)
	assert.NotContains(t, output, "superSecretPassword")
}

func TestSecret_UnderlyingValueUnchanged(t *testing.T) {
	secret := Secret("superSecretPassword")
	require.Equal(t, "superSecretPassword", string(secret))

	ref := Secret("secrets:resource:key")
	require.Equal(t, "secrets:resource:key", string(ref))
}

func TestSecret_IsRef(t *testing.T) {
	assert.True(t, Secret("secrets:resource:key").isRef())
	assert.False(t, Secret("secrets:foo").isRef())
	assert.False(t, Secret("plain-password").isRef())
	assert.False(t, Secret("").isRef())
}

func TestSecret_Validate(t *testing.T) {
	t.Run("secret ref without agent", func(t *testing.T) {
		err := Secret("secrets:asbackup:psw").Validate(false)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrSecretValidation)
		require.ErrorContains(t, err, "secrets:asbackup:psw")
	})

	t.Run("secret ref with agent", func(t *testing.T) {
		require.NoError(t, Secret("secrets:resource:key").Validate(true))
	})

	t.Run("plain value", func(t *testing.T) {
		require.NoError(t, Secret("plain-password").Validate(false))
	})

	t.Run("malformed secret ref", func(t *testing.T) {
		err := Secret("secrets:foo").Validate(true)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrSecretValidation)
		require.ErrorContains(t, err, "secrets:foo")
		require.ErrorContains(t, err, "secrets:<resource>:<key>")
	})
}

func TestSecret_DisplayString_MalformedRef(t *testing.T) {
	assert.Equal(t, RedactedSecret, Secret("secrets:foo").DisplayString())
}
