package dto

import (
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackupServiceConfig_Compare(t *testing.T) {
	tests := []struct {
		name    string
		current ServiceConfig
		other   ServiceConfig
		errors  []string
	}{
		{
			name: "identical configs",
			current: ServiceConfig{
				HTTPServer: &HTTPServerConfig{
					Address: "localhost",
					Port:    ptr.Of(Port(8080)),
				},
				Logger: &LoggerConfig{
					Level:  "INFO",
					Format: "JSON",
				},
			},
			other: ServiceConfig{
				HTTPServer: &HTTPServerConfig{
					Address: "localhost",
					Port:    ptr.Of(Port(8080)),
				},
				Logger: &LoggerConfig{
					Level:  "INFO",
					Format: "JSON",
				},
			},
			errors: nil,
		},
		{
			name: "changed http server config",
			current: ServiceConfig{
				HTTPServer: &HTTPServerConfig{
					Address: "localhost",
					Port:    ptr.Of(Port(8080)),
				},
			},
			other: ServiceConfig{
				HTTPServer: &HTTPServerConfig{
					Address: "0.0.0.0",
					Port:    ptr.Of(Port(9090)),
				},
			},
			errors: []string{
				"Address changed: localhost -> 0.0.0.0",
				"Port changed: 8080 -> 9090",
			},
		},
		{
			name: "changed logger config",
			current: ServiceConfig{
				Logger: &LoggerConfig{
					Level:        "INFO",
					Format:       "JSON",
					StdoutWriter: ptr.Of(true),
				},
			},
			other: ServiceConfig{
				Logger: &LoggerConfig{
					Level:        "DEBUG",
					Format:       "PLAIN",
					StdoutWriter: ptr.Of(false),
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
			assert.Len(t, strings.Split(errStr, "\n"), len(tt.errors))
			for _, substr := range tt.errors {
				assert.Contains(t, errStr, substr)
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
			assert.Len(t, strings.Split(errStr, "\n"), len(tt.errors))
			for _, substr := range tt.errors {
				assert.Contains(t, errStr, substr)
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
				Tps:       ptr.Of(100),
				Size:      ptr.Of(1000),
				WhiteList: []string{"127.0.0.1", "localhost"},
			},
			other: &RateLimiterConfig{
				Tps:       ptr.Of(100),
				Size:      ptr.Of(1000),
				WhiteList: []string{"127.0.0.1", "localhost"},
			},
			errors: nil,
		},
		{
			name: "changed values",
			current: &RateLimiterConfig{
				Tps:       ptr.Of(100),
				Size:      ptr.Of(1000),
				WhiteList: []string{"127.0.0.1"},
			},
			other: &RateLimiterConfig{
				Tps:       ptr.Of(200),
				Size:      ptr.Of(2000),
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
			assert.Len(t, strings.Split(errStr, "\n"), len(tt.errors))
			for _, substr := range tt.errors {
				assert.Contains(t, errStr, substr)
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
				Address:      "localhost",
				Port:         ptr.Of(Port(8080)),
				ContextPath:  "/api",
				Timeout:      ptr.Of(int64(5000)),
				ReadTimeout:  ptr.Of(int64(30000)),
				WriteTimeout: ptr.Of(int64(60000)),
				IdleTimeout:  ptr.Of(int64(120000)),
			},
			other: &HTTPServerConfig{
				Address:      "localhost",
				Port:         ptr.Of(Port(8080)),
				ContextPath:  "/api",
				Timeout:      ptr.Of(int64(5000)),
				ReadTimeout:  ptr.Of(int64(30000)),
				WriteTimeout: ptr.Of(int64(60000)),
				IdleTimeout:  ptr.Of(int64(120000)),
			},
			errors: nil,
		},
		{
			name: "all fields changed",
			current: &HTTPServerConfig{
				Address:      "localhost",
				Port:         ptr.Of(Port(8080)),
				ContextPath:  "/api",
				Timeout:      ptr.Of(int64(5000)),
				ReadTimeout:  ptr.Of(int64(30000)),
				WriteTimeout: ptr.Of(int64(60000)),
				IdleTimeout:  ptr.Of(int64(120000)),
			},
			other: &HTTPServerConfig{
				Address:      "0.0.0.0",
				Port:         ptr.Of(Port(9090)),
				ContextPath:  "/v1/api",
				Timeout:      ptr.Of(int64(10000)),
				ReadTimeout:  ptr.Of(int64(45000)),
				WriteTimeout: ptr.Of(int64(90000)),
				IdleTimeout:  ptr.Of(int64(180000)),
			},
			errors: []string{
				"Address changed: localhost -> 0.0.0.0",
				"Port changed: 8080 -> 9090",
				"ContextPath changed: /api -> /v1/api",
				"Timeout changed: 5000 -> 10000",
				"ReadTimeout changed: 30000 -> 45000",
				"WriteTimeout changed: 60000 -> 90000",
				"IdleTimeout changed: 120000 -> 180000",
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
			assert.Len(t, strings.Split(errStr, "\n"), len(tt.errors))
			for _, substr := range tt.errors {
				assert.Contains(t, errStr, substr)
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
				Level:        "INFO",
				Format:       "JSON",
				StdoutWriter: ptr.Of(true),
				FileWriter: &FileLoggerConfig{
					Filename: "app.log",
					MaxSize:  100,
				},
			},
			other: &LoggerConfig{
				Level:        "INFO",
				Format:       "JSON",
				StdoutWriter: ptr.Of(true),
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
				Level:        "INFO",
				Format:       "JSON",
				StdoutWriter: ptr.Of(true),
				FileWriter: &FileLoggerConfig{
					Filename: "app.log",
				},
			},
			other: &LoggerConfig{
				Level:        "DEBUG",
				Format:       "PLAIN",
				StdoutWriter: ptr.Of(false),
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
			assert.Len(t, strings.Split(errStr, "\n"), len(tt.errors))
			for _, substr := range tt.errors {
				assert.Contains(t, errStr, substr)
			}
		})
	}
}
