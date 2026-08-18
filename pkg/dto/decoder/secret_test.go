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
}

func TestSecret_GoString(t *testing.T) {
	assert.Equal(t, `decoder.Secret("")`, decoder.Secret("").GoString())
	assert.Equal(t, `decoder.Secret("[secret]")`, decoder.Secret("superSecretPassword").GoString())

	output := fmt.Sprintf("%#v", decoder.Secret("superSecretPassword"))
	assert.NotContains(t, output, "superSecretPassword")
	assert.Contains(t, output, "[secret]")
}

func TestSecret_LogValue(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	logger.Info("credentials", slog.Any("password", decoder.Secret("superSecretPassword")))

	assert.Contains(t, buf.String(), `"password":"`+decoder.RedactedSecret+`"`)
	assert.NotContains(t, buf.String(), "superSecretPassword")
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
}
