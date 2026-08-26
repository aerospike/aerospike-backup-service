package model

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/optional"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/aerospike/backup-go/models"
)

const (
	DefaultSocketTimeout = 10 * time.Minute
	DefaultPartSize      = 50 * 1024 * 1024
)

// defaultListener represents the listen settings shared by the HTTP and HTTPS servers.
var defaultListener = ListenerConfig{
	Address: "0.0.0.0",
	Rate: &RateLimiterConfig{
		Tps:       ptr.Of(1024),
		Size:      ptr.Of(1024),
		WhiteList: []string{},
	},
	ContextPath:  "/",
	Timeout:      ptr.Of(5 * time.Second),
	ReadTimeout:  ptr.Of(30 * time.Second),
	WriteTimeout: ptr.Of(60 * time.Second),
	IdleTimeout:  ptr.Of(120 * time.Second),
}

// defaultConfig represents default configuration values.
var defaultConfig = struct {
	http          ServerConfigHTTP
	https         ServerConfigHTTPS
	logger        LoggerConfig
	backupPolicy  BackupPolicy
	restorePolicy RestorePolicy
	credentials   Credentials
}{
	http: ServerConfigHTTP{
		ListenerConfig: defaultListener,
		Port:           NewPort(8080),
	},
	https: ServerConfigHTTPS{
		ListenerConfig: defaultListener,
		Port:           NewPort(8443),
		MinVersion:     TLSMinVersion12,
		ClientAuth:     TLSClientAuthNone,
	},
	logger: LoggerConfig{
		Level:        LogLevelInfo,
		Format:       LogFormatPlain,
		StdoutWriter: ptr.Of(true),
		FileWriter: &FileLoggerConfig{
			MaxSize:    100,
			MaxAge:     7,
			MaxBackups: 3,
		},
	},
	backupPolicy: BackupPolicy{
		RetryPolicy: &RetryPolicy{
			BaseTimeout: optional.Of(1 * time.Minute),
			MaxRetries:  optional.Of(5),
			Multiplier:  ptr.Of(1.5),
		},
		Parallel:              ptr.Of(8),
		FileLimit:             ptr.Of(250),
		Sealed:                ptr.Of(false),
		Compact:               ptr.Of(false),
		UseCompression:        ptr.Of(false),
		ConcurrentIncremental: ptr.Of(false),
	},
	restorePolicy: RestorePolicy{
		Parallel: ptr.Of(8),
		RetryPolicy: &RetryPolicy{
			BaseTimeout: optional.Of(2 * time.Second),
			MaxRetries:  optional.Of(5),
			Multiplier:  ptr.Of(2.0),
		},
		MaxAsyncBatches: ptr.Of(128),
		BatchSize:       ptr.Of(128),
	},
	credentials: Credentials{
		AuthMode: AuthModeInternal,
	},
}

// StorageRetryPolicy defines the global retry policy for storage operations.
// It is used in GCP, Azure and S3 client configurations.
var StorageRetryPolicy = struct {
	models.RetryPolicy
	MaxBackoffDuration time.Duration
	MaxRequestTimeout  time.Duration
}{
	RetryPolicy: models.RetryPolicy{
		BaseTimeout: 1 * time.Second,
		MaxRetries:  100,
		Multiplier:  1.1,
	},
	MaxBackoffDuration: 2 * time.Minute,
	MaxRequestTimeout:  10 * time.Minute,
}

var ScanRetryPolicy = &models.RetryPolicy{
	BaseTimeout: 1000 * time.Millisecond,
	Multiplier:  1.5,
	MaxRetries:  10,
}

var InfoRetryPolicy = &models.RetryPolicy{
	BaseTimeout: 1000 * time.Millisecond,
	Multiplier:  1.5,
	MaxRetries:  10,
}
