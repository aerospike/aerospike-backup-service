package dto

import (
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackupServiceConfig_Compare(t *testing.T) {
	tests := []struct {
		name    string
		current BackupServiceConfig
		other   BackupServiceConfig
		errors  []string
	}{
		{
			name: "identical configs",
			current: BackupServiceConfig{
				HTTPServer: &HTTPServerConfig{
					Address: util.Ptr("localhost"),
					Port:    util.Ptr(8080),
				},
				Logger: &LoggerConfig{
					Level:  util.Ptr("INFO"),
					Format: util.Ptr("JSON"),
				},
			},
			other: BackupServiceConfig{
				HTTPServer: &HTTPServerConfig{
					Address: util.Ptr("localhost"),
					Port:    util.Ptr(8080),
				},
				Logger: &LoggerConfig{
					Level:  util.Ptr("INFO"),
					Format: util.Ptr("JSON"),
				},
			},
			errors: nil,
		},
		{
			name: "changed http server config",
			current: BackupServiceConfig{
				HTTPServer: &HTTPServerConfig{
					Address: util.Ptr("localhost"),
					Port:    util.Ptr(8080),
				},
			},
			other: BackupServiceConfig{
				HTTPServer: &HTTPServerConfig{
					Address: util.Ptr("0.0.0.0"),
					Port:    util.Ptr(9090),
				},
			},
			errors: []string{
				"Address changed: localhost -> 0.0.0.0",
				"Port changed: 8080 -> 9090",
			},
		},
		{
			name: "changed logger config",
			current: BackupServiceConfig{
				Logger: &LoggerConfig{
					Level:        util.Ptr("INFO"),
					Format:       util.Ptr("JSON"),
					StdoutWriter: util.Ptr(true),
				},
			},
			other: BackupServiceConfig{
				Logger: &LoggerConfig{
					Level:        util.Ptr("DEBUG"),
					Format:       util.Ptr("PLAIN"),
					StdoutWriter: util.Ptr(false),
				},
			},
			errors: []string{
				"Level changed: INFO -> DEBUG",
				"Format changed: JSON -> PLAIN",
				"StdoutWriter changed: true -> false",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.current.Compare(tt.other)
			if tt.errors == nil {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)

			errStr := err.Error()
			assert.Equal(t, len(tt.errors), len(strings.Split(errStr, "\n")))
			for _, substr := range tt.errors {
				if !strings.Contains(errStr, substr) {
					t.Errorf("error message %q should contain %q", errStr, substr)
				}
			}
		})
	}
}

func TestFileLoggerConfig_Compare(t *testing.T) {
	tests := []struct {
		name    string
		current *FileLoggerConfig
		other   *FileLoggerConfig
		errors  []string
	}{
		{
			name: "identical configs",
			current: &FileLoggerConfig{
				Filename:   "app.log",
				MaxSize:    100,
				MaxAge:     7,
				MaxBackups: 5,
				Compress:   true,
			},
			other: &FileLoggerConfig{
				Filename:   "app.log",
				MaxSize:    100,
				MaxAge:     7,
				MaxBackups: 5,
				Compress:   true,
			},
			errors: nil,
		},
		{
			name: "all fields changed",
			current: &FileLoggerConfig{
				Filename:   "app.log",
				MaxSize:    100,
				MaxAge:     7,
				MaxBackups: 5,
				Compress:   true,
			},
			other: &FileLoggerConfig{
				Filename:   "new.log",
				MaxSize:    200,
				MaxAge:     14,
				MaxBackups: 10,
				Compress:   false,
			},
			errors: []string{
				"Filename changed: app.log -> new.log",
				"MaxSize changed: 100 -> 200",
				"MaxAge changed: 7 -> 14",
				"MaxBackups changed: 5 -> 10",
				"Compress changed: true -> false",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.current.Compare(tt.other)
			if tt.errors == nil {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)

			errStr := err.Error()
			assert.Equal(t, len(tt.errors), len(strings.Split(errStr, "\n")))
			for _, substr := range tt.errors {
				if !strings.Contains(errStr, substr) {
					t.Errorf("error message %q should contain %q", errStr, substr)
				}
			}
		})
	}
}

func TestRateLimiterConfig_Compare(t *testing.T) {
	tests := []struct {
		name    string
		current *RateLimiterConfig
		other   *RateLimiterConfig
		errors  []string
	}{
		{
			name: "identical configs",
			current: &RateLimiterConfig{
				Tps:       util.Ptr(100),
				Size:      util.Ptr(1000),
				WhiteList: []string{"127.0.0.1", "localhost"},
			},
			other: &RateLimiterConfig{
				Tps:       util.Ptr(100),
				Size:      util.Ptr(1000),
				WhiteList: []string{"127.0.0.1", "localhost"},
			},
			errors: nil,
		},
		{
			name: "changed values",
			current: &RateLimiterConfig{
				Tps:       util.Ptr(100),
				Size:      util.Ptr(1000),
				WhiteList: []string{"127.0.0.1"},
			},
			other: &RateLimiterConfig{
				Tps:       util.Ptr(200),
				Size:      util.Ptr(2000),
				WhiteList: []string{"127.0.0.1", "localhost"},
			},
			errors: []string{
				"Tps changed: 100 -> 200",
				"Size changed: 1000 -> 2000",
				"WhiteList length changed: 1 -> 2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.current.Compare(tt.other)
			if tt.errors == nil {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)

			errStr := err.Error()
			assert.Equal(t, len(tt.errors), len(strings.Split(errStr, "\n")))
			for _, substr := range tt.errors {
				if !strings.Contains(errStr, substr) {
					t.Errorf("error message %q should contain %q", errStr, substr)
				}
			}
		})
	}
}

func TestHTTPServerConfig_Compare(t *testing.T) {
	tests := []struct {
		name    string
		current *HTTPServerConfig
		other   *HTTPServerConfig
		errors  []string
	}{
		{
			name: "identical configs",
			current: &HTTPServerConfig{
				Address:     util.Ptr("localhost"),
				Port:        util.Ptr(8080),
				ContextPath: util.Ptr("/api"),
				Timeout:     util.Ptr(5000),
			},
			other: &HTTPServerConfig{
				Address:     util.Ptr("localhost"),
				Port:        util.Ptr(8080),
				ContextPath: util.Ptr("/api"),
				Timeout:     util.Ptr(5000),
			},
			errors: nil,
		},
		{
			name: "all fields changed",
			current: &HTTPServerConfig{
				Address:     util.Ptr("localhost"),
				Port:        util.Ptr(8080),
				ContextPath: util.Ptr("/api"),
				Timeout:     util.Ptr(5000),
			},
			other: &HTTPServerConfig{
				Address:     util.Ptr("0.0.0.0"),
				Port:        util.Ptr(9090),
				ContextPath: util.Ptr("/v1/api"),
				Timeout:     util.Ptr(10000),
			},
			errors: []string{
				"Address changed: localhost -> 0.0.0.0",
				"Port changed: 8080 -> 9090",
				"ContextPath changed: /api -> /v1/api",
				"Timeout changed: 5000 -> 10000",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.current.Compare(tt.other)
			if tt.errors == nil {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)

			errStr := err.Error()
			assert.Equal(t, len(tt.errors), len(strings.Split(errStr, "\n")))
			for _, substr := range tt.errors {
				if !strings.Contains(errStr, substr) {
					t.Errorf("error message %q should contain %q", errStr, substr)
				}
			}
		})
	}
}

func TestLoggerConfig_Compare(t *testing.T) {
	tests := []struct {
		name    string
		current *LoggerConfig
		other   *LoggerConfig
		errors  []string
	}{
		{
			name: "identical configs",
			current: &LoggerConfig{
				Level:        util.Ptr("INFO"),
				Format:       util.Ptr("JSON"),
				StdoutWriter: util.Ptr(true),
				FileWriter: &FileLoggerConfig{
					Filename: "app.log",
					MaxSize:  100,
				},
			},
			other: &LoggerConfig{
				Level:        util.Ptr("INFO"),
				Format:       util.Ptr("JSON"),
				StdoutWriter: util.Ptr(true),
				FileWriter: &FileLoggerConfig{
					Filename: "app.log",
					MaxSize:  100,
				},
			},
			errors: nil,
		},
		{
			name: "all fields changed",
			current: &LoggerConfig{
				Level:        util.Ptr("INFO"),
				Format:       util.Ptr("JSON"),
				StdoutWriter: util.Ptr(true),
				FileWriter: &FileLoggerConfig{
					Filename: "app.log",
				},
			},
			other: &LoggerConfig{
				Level:        util.Ptr("DEBUG"),
				Format:       util.Ptr("PLAIN"),
				StdoutWriter: util.Ptr(false),
				FileWriter: &FileLoggerConfig{
					Filename: "new.log",
				},
			},
			errors: []string{
				"Level changed: INFO -> DEBUG",
				"Format changed: JSON -> PLAIN",
				"StdoutWriter changed: true -> false",
				"Filename changed: app.log -> new.log",
			},
		},
		{
			name: "empty configs",
			current: &LoggerConfig{
				FileWriter: &FileLoggerConfig{},
			},
			other: &LoggerConfig{
				FileWriter: &FileLoggerConfig{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.current.Compare(tt.other)
			if tt.errors == nil {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)

			errStr := err.Error()
			assert.Equal(t, len(tt.errors), len(strings.Split(errStr, "\n")))
			for _, substr := range tt.errors {
				if !strings.Contains(errStr, substr) {
					t.Errorf("error message %q should contain %q", errStr, substr)
				}
			}
		})
	}
}
