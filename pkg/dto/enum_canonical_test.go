package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanonicalEnum(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		allowed []string
		want    string
		ok      bool
	}{
		{
			name:    "canonical",
			input:   "INFO",
			allowed: []string{"TRACE", "INFO", "WARN"},
			want:    "INFO",
			ok:      true,
		},
		{
			name:    "lowercase",
			input:   "info",
			allowed: []string{"TRACE", "INFO", "WARN"},
			want:    "INFO",
			ok:      true,
		},
		{
			name:    "trimmed",
			input:   "  Warn  ",
			allowed: []string{"TRACE", "INFO", "WARN", "WARNING"},
			want:    "WARN",
			ok:      true,
		},
		{
			name:    "longer alias first-match",
			input:   "warning",
			allowed: []string{"WARN", "WARNING"},
			want:    "WARNING",
			ok:      true,
		},
		{
			name:    "empty",
			input:   "",
			allowed: []string{"INFO"},
			want:    "",
			ok:      true,
		},
		{
			name:    "unknown",
			input:   "verbose",
			allowed: []string{"INFO"},
			want:    "verbose",
			ok:      false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := canonicalEnum(test.input, test.allowed)
			assert.Equal(t, test.ok, ok)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestEnumToModelCanonicalizes(t *testing.T) {
	t.Parallel()

	assert.Equal(t, model.AuthModeInternal, AuthMode("internal").ToModel())
	assert.Equal(t, model.CompressionModeZSTD, CompressionMode("Zstd").ToModel())
	assert.Equal(t, model.EncryptionModeAES256, EncryptionMode("aes256").ToModel())
	assert.Equal(t, model.LogLevelInfo, LogLevel("info").ToModel())
	assert.Equal(t, model.LogLevelWarn, LogLevel("warn").ToModel())
	assert.Equal(t, model.LogLevelWarning, LogLevel("Warning").ToModel())
	assert.Equal(t, model.LogFormatJSON, LogFormat("json").ToModel())
	assert.Equal(t, model.ConnectionTypeTCP, ConnectionType("TCP").ToModel())
	assert.Equal(t, model.S3LogLevel("FATAL"), S3LogLevel("fatal").ToModel())
	assert.Equal(t, model.TLSClientAuthRequireAndVerify, TLSClientAuth("Require-And-Verify").ToModel())
	require.NotNil(t, TimestampFormat("eu").ToModel())
	assert.Equal(t, model.TimestampFormatEU, *TimestampFormat("eu").ToModel())
}

func TestS3LogLevelValidate(t *testing.T) {
	tests := []struct {
		name    string
		level   S3LogLevel
		wantErr bool
	}{
		{
			name:  "canonical",
			level: S3LogLevelFatal,
		},
		{
			name:  "lowercase",
			level: "debug",
		},
		{
			name:  "empty",
			level: "",
		},
		{
			name:    "unsupported",
			level:   "VERBOSE",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.level.Validate()
			if test.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
