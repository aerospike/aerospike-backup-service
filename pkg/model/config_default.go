package model

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/util"
	"github.com/aerospike/backup-go/models"
)

type backupPolicy struct {
	maxRetries int32
	retryDelay int32
	sealed     bool
}

// defaultConfig represents default configuration values.
var defaultConfig = struct {
	http         HTTPServerConfig
	logger       LoggerConfig
	backupPolicy backupPolicy
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
		retryDelay: 60_000, // default retry delay is 1 minute
		maxRetries: 3,
	},
}

var defaultRetry = &models.RetryPolicy{
	BaseTimeout: 2 * time.Second,
	MaxRetries:  5,
	Multiplier:  2,
}
