package model

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/util"
	"github.com/aerospike/backup-go/models"
)

type backupPolicy struct {
	retryPolicy models.RetryPolicy
	sealed      bool
}

// defaultConfig represents default configuration values.
var defaultConfig = struct {
	http          HTTPServerConfig
	logger        LoggerConfig
	backupPolicy  backupPolicy
	restorePolicy RestorePolicy
}{
	http: HTTPServerConfig{
		Address: util.Ptr("0.0.0.0"),
		Port:    util.Ptr(8080),
		Rate: &RateLimiterConfig{
			Tps:       util.Ptr(1024),
			Size:      util.Ptr(1024),
			WhiteList: []string{},
		},
		ContextPath: util.Ptr("/"),
		Timeout:     util.Ptr(5000),
	},
	logger: LoggerConfig{
		Level:        util.Ptr("DEBUG"),
		Format:       util.Ptr("PLAIN"),
		StdoutWriter: util.Ptr(true),
	},
	backupPolicy: backupPolicy{
		retryPolicy: models.RetryPolicy{
			BaseTimeout: 1 * time.Minute,
			MaxRetries:  5,
			Multiplier:  1,
		},
	},
	restorePolicy: RestorePolicy{
		Parallel: util.Ptr(20),
		RetryPolicy: &models.RetryPolicy{
			BaseTimeout: 2 * time.Second,
			MaxRetries:  5,
			Multiplier:  2,
		},
		MaxAsyncBatches: util.Ptr(640),
		BatchSize:       util.Ptr(128),
		Timeout:         util.Ptr(int32(10_000)),
	},
}
