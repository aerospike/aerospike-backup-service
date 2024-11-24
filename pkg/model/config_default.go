package model

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/util"
	"github.com/aerospike/backup-go/models"
)

// defaultConfig represents default configuration values.
var defaultConfig = struct {
	http          HTTPServerConfig
	logger        LoggerConfig
	backupPolicy  BackupPolicy
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
	backupPolicy: BackupPolicy{
		RetryPolicy: &models.RetryPolicy{
			BaseTimeout: 1 * time.Minute,
			MaxRetries:  5,
			Multiplier:  1,
		},
		Parallel:      util.Ptr(1),
		FileLimit:     util.Ptr(250),
		TotalTimeout:  util.Ptr(time.Duration(0)),
		SocketTimeout: util.Ptr(10 * time.Second),
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
		Timeout:         util.Ptr(10 * time.Second),
	},
}
