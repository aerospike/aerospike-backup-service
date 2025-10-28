package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/require"
)

func TestBackupServiceConfig_Validate_Success(t *testing.T) {
	cfg := &ServiceConfig{
		HTTPServer: &HTTPServerConfig{ContextPath: ptr.Of("/")},
		Logger:     &LoggerConfig{Level: ptr.Of("INFO")},
	}

	err := cfg.Validate()
	require.NoError(t, err)
}

func TestBackupServiceConfig_Validate_PropagatesHTTPServerError(t *testing.T) {
	cfg := &ServiceConfig{
		HTTPServer: &HTTPServerConfig{ContextPath: ptr.Of("FOO")}, // missing leading slash => invalid
	}

	err := cfg.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "`http` validation error")
}

func TestBackupServiceConfig_Validate_PropagatesLoggerError(t *testing.T) {
	cfg := &ServiceConfig{
		Logger: &LoggerConfig{Level: ptr.Of("FOO")},
	}

	err := cfg.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid logger level")
}

func TestBackupServiceConfig_Validate_InvalidTimestampFormat(t *testing.T) {
	cfg := &ServiceConfig{
		Backup: &BackupCommonConfig{TimestampFormat: ptr.Of("UK")},
	}

	err := cfg.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "is not a valid timestamp-format")
}

func TestBackupServiceConfig_Validate_ValidTimestampFormats(t *testing.T) {
	for _, v := range []string{"ISO", "US", "EU", "iso", "us", "eu"} {
		cfg := &ServiceConfig{
			Backup: &BackupCommonConfig{TimestampFormat: &v},
		}
		err := cfg.Validate()
		require.NoError(t, err)
	}
}
