package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/require"
)

func TestBackupServiceConfig_Validate_Success(t *testing.T) {
	cfg := &ServiceConfig{
		ServerHTTP: &ServerConfigHTTP{ListenerConfig: ListenerConfig{ContextPath: "/"}},
		Logger:     &LoggerConfig{Level: "INFO"},
	}

	err := cfg.Validate(ValidationDefault)
	require.NoError(t, err)
}

func TestBackupServiceConfig_Validate_PropagatesServerHTTPError(t *testing.T) {
	cfg := &ServiceConfig{
		// missing leading slash => invalid
		ServerHTTP: &ServerConfigHTTP{ListenerConfig: ListenerConfig{ContextPath: "FOO"}},
	}

	err := cfg.Validate(ValidationDefault)
	require.Error(t, err)
	require.Contains(t, err.Error(), "`http` validation error")
}

func TestBackupServiceConfig_Validate_PropagatesLoggerError(t *testing.T) {
	cfg := &ServiceConfig{
		Logger: &LoggerConfig{Level: "FOO"},
	}

	err := cfg.Validate(ValidationDefault)
	require.Error(t, err)
	require.Contains(t, err.Error(), "is not a valid level")
}

func TestBackupServiceConfig_Validate_InvalidTimestampFormat(t *testing.T) {
	cfg := &ServiceConfig{
		Backup: &BackupCommonConfig{TimestampFormat: TimestampFormat("UK")},
	}

	err := cfg.Validate(ValidationDefault)
	require.Error(t, err)
	require.Contains(t, err.Error(), "is not a valid timestamp-format")
}

func TestBackupServiceConfig_Validate_ValidTimestampFormats(t *testing.T) {
	for _, v := range []TimestampFormat{"", TimestampFormatISO, TimestampFormatUS, TimestampFormatEU} {
		cfg := &ServiceConfig{
			Backup: &BackupCommonConfig{TimestampFormat: v},
		}
		err := cfg.Validate(ValidationDefault)
		require.NoError(t, err)
	}
}

func TestBackupServiceConfig_Validate_ScheduleTimezone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		wantErr string
	}{
		{name: "omitted", value: ""},
		{name: "utc", value: "UTC"},
		{name: "local", value: "local"},
		{name: "iana", value: "America/New_York"},
		{name: "EST rejected", value: "EST", wantErr: "EST"},
		{name: "unknown name", value: "Not/AZone", wantErr: "Not/AZone"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &ServiceConfig{
				Backup: &BackupCommonConfig{ScheduleTimezone: tt.value},
			}
			err := cfg.Validate(ValidationDefault)
			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestBackupCommonConfig_Compare_ScheduleTimezone(t *testing.T) {
	t.Parallel()

	current := &BackupCommonConfig{ScheduleTimezone: "UTC"}
	other := &BackupCommonConfig{ScheduleTimezone: "America/New_York"}

	err := current.Compare(other)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ScheduleTimezone changed")
}

func TestBackupServiceConfig_Validate_CaseInsensitiveEnums(t *testing.T) {
	cfg := &ServiceConfig{
		Logger: &LoggerConfig{Level: "info", Format: "json"},
		Backup: &BackupCommonConfig{TimestampFormat: "iso"},
	}

	err := cfg.Validate(ValidationDefault)
	require.NoError(t, err)
}

func TestServiceConfigValidateListeners(t *testing.T) {
	certs := setupTestCertificates(t)

	tests := []struct {
		name    string
		config  *ServiceConfig
		wantErr string
	}{
		{
			name:   "default HTTP listener",
			config: &ServiceConfig{},
		},
		{
			name: "HTTPS listener with HTTP disabled",
			config: &ServiceConfig{
				ServerHTTP: &ServerConfigHTTP{ListenerConfig: ListenerConfig{Disabled: true}},
				ServerHTTPS: &ServerConfigHTTPS{
					CertFile: certs.certFile,
					KeyFile:  certs.keyFile,
				},
			},
		},
		{
			name: "both listeners enabled on different ports",
			config: &ServiceConfig{
				ServerHTTP: &ServerConfigHTTP{Port: ptr.Of(Port(8080))},
				ServerHTTPS: &ServerConfigHTTPS{
					Port:     ptr.Of(Port(8443)),
					CertFile: certs.certFile,
					KeyFile:  certs.keyFile,
				},
			},
		},
		{
			name: "HTTP disabled and HTTPS absent",
			config: &ServiceConfig{
				ServerHTTP: &ServerConfigHTTP{ListenerConfig: ListenerConfig{Disabled: true}},
			},
			wantErr: "service.http and service.https cannot both be disabled",
		},
		{
			name: "both listeners disabled",
			config: &ServiceConfig{
				ServerHTTP:  &ServerConfigHTTP{ListenerConfig: ListenerConfig{Disabled: true}},
				ServerHTTPS: &ServerConfigHTTPS{ListenerConfig: ListenerConfig{Disabled: true}},
			},
			wantErr: "service.http and service.https cannot both be disabled",
		},
		{
			name: "both listeners use same explicit port",
			config: &ServiceConfig{
				ServerHTTP: &ServerConfigHTTP{Port: ptr.Of(Port(8443))},
				ServerHTTPS: &ServerConfigHTTPS{
					Port:     ptr.Of(Port(8443)),
					CertFile: certs.certFile,
					KeyFile:  certs.keyFile,
				},
			},
			wantErr: "cannot use the same port 8443",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.config.Validate(ValidationSkipTLSFiles)
			if test.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, test.wantErr)
		})
	}
}
