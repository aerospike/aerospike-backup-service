package model

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	"github.com/aerospike/backup-go/models"
)

const DefaultSocketTimeout = 10 * time.Minute

// defaultConfig represents default configuration values.
var defaultConfig = struct {
	http          HTTPServerConfig
	logger        LoggerConfig
	backupPolicy  BackupPolicy
	restorePolicy RestorePolicy
	xdrConfig     XDRConfig
}{
	http: HTTPServerConfig{
		Address: util.Ptr("0.0.0.0"),
		Port:    NewPort(8080),
		Rate: &RateLimiterConfig{
			Tps:       util.Ptr(1024),
			Size:      util.Ptr(1024),
			WhiteList: []string{},
		},
		ContextPath: util.Ptr("/"),
		Timeout:     util.Ptr(5 * time.Second),
	},
	logger: LoggerConfig{
		Level:        util.Ptr("DEBUG"),
		Format:       util.Ptr("PLAIN"),
		StdoutWriter: util.Ptr(true),
		FileWriter: &FileLoggerConfig{
			MaxSize: 0,
		},
	},
	backupPolicy: BackupPolicy{
		RetryPolicy: &models.RetryPolicy{
			BaseTimeout: 1 * time.Minute,
			MaxRetries:  5,
			Multiplier:  1,
		},
		Parallel:  util.Ptr(8),
		FileLimit: util.Ptr(250),
	},
	restorePolicy: RestorePolicy{
		Parallel: util.Ptr(20),
		RetryPolicy: &models.RetryPolicy{
			BaseTimeout: 2 * time.Second,
			MaxRetries:  5,
			Multiplier:  2,
		},
		MaxAsyncBatches: util.Ptr(128),
		BatchSize:       util.Ptr(128),
	},
	xdrConfig: XDRConfig{
		MaxConns:        util.Ptr(100),
		ReadTimeout:     util.Ptr(1 * time.Second),
		WriteTimeout:    util.Ptr(1 * time.Second),
		StartTimeout:    util.Ptr(30 * time.Second),
		PollingPeriod:   util.Ptr(1 * time.Second),
		ResultQueueSize: util.Ptr(256),
		AckQueueSize:    util.Ptr(256),
		InfoRetryPolicy: &models.RetryPolicy{
			BaseTimeout: 2 * time.Second,
			MaxRetries:  5,
			Multiplier:  2,
		},
	},
}

var StorageRetryPolicy = struct {
	models.RetryPolicy
	MaxDuration time.Duration
}{
	RetryPolicy: models.RetryPolicy{
		BaseTimeout: 1 * time.Second,
		MaxRetries:  100,
		Multiplier:  1.1,
	},
	MaxDuration: 2 * time.Minute,
}
