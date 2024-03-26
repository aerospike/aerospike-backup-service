package model

import "github.com/aerospike/backup/pkg/util"

type backupPolicy struct {
	sealed bool
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
	},
	logger: LoggerConfig{
		Level:        util.Ptr("DEBUG"),
		Format:       util.Ptr("PLAIN"),
		StdoutWriter: util.Ptr(true),
	},
}
