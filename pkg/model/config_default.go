package model

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/optional"
	"github.com/aerospike/backup-go/models"
)

const (
	DefaultSocketTimeout = 10 * time.Minute
	DefaultPartSize      = 50 * 1024 * 1024
)

var DefaultScheduleTimezone = time.UTC

// defaultListener represents the listen settings shared by the HTTP and HTTPS servers.
var defaultListener = ListenerConfig{
	Address: "0.0.0.0",
	Rate: &RateLimiterConfig{
		Tps:       new(1024),
		Size:      new(1024),
		WhiteList: []string{},
	},
	ContextPath:  "/",
	Timeout:      new(5 * time.Second),
	ReadTimeout:  new(30 * time.Second),
	WriteTimeout: new(60 * time.Second),
	IdleTimeout:  new(120 * time.Second),
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
		ListenerConfig: ListenerConfig{
			Disabled:     true,
			Address:      defaultListener.Address,
			Rate:         defaultListener.Rate,
			ContextPath:  defaultListener.ContextPath,
			Timeout:      defaultListener.Timeout,
			ReadTimeout:  defaultListener.ReadTimeout,
			WriteTimeout: defaultListener.WriteTimeout,
			IdleTimeout:  defaultListener.IdleTimeout,
		},
		Port:       NewPort(8443),
		MinVersion: TLSMinVersion12,
		ClientAuth: TLSClientAuthNone,
	},
	logger: LoggerConfig{
		Level:        LogLevelInfo,
		Format:       LogFormatPlain,
		StdoutWriter: new(true),
		FileWriter: &FileLoggerConfig{
			MaxSize: 100,
		},
	},
	backupPolicy: BackupPolicy{
		RetryPolicy: &RetryPolicy{
			BaseTimeout: optional.Of(1 * time.Minute),
			MaxRetries:  optional.Of(5),
			Multiplier:  new(1.5),
		},
		Parallel:              new(8),
		FileLimit:             new(250),
		Sealed:                new(false),
		Compact:               new(false),
		WithClusterConfig:     new(false),
		UseCompression:        new(false),
		ConcurrentIncremental: new(false),
		IncrMode:              IncrModeDifferential,
	},
	restorePolicy: RestorePolicy{
		Parallel: new(8),
		RetryPolicy: &RetryPolicy{
			BaseTimeout: optional.Of(2 * time.Second),
			MaxRetries:  optional.Of(5),
			Multiplier:  new(2.0),
		},
		MaxAsyncBatches: new(128),
		BatchSize:       new(128),
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
