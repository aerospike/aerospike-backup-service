package model

// BackupServiceConfig represents the backup service configuration properties.
type BackupServiceConfig struct {
	// HTTPServer is the backup service HTTP server configuration.
	HTTPServer *HTTPServerConfig
	// Logger is the backup service logger configuration.
	Logger *LoggerConfig
}

func (c BackupServiceConfig) GetHTTPServerOrDefault() *HTTPServerConfig {
	if c.HTTPServer != nil {
		return c.HTTPServer
	}

	return &defaultConfig.http
}

func (c BackupServiceConfig) GetLoggerOrDefault() *LoggerConfig {
	if c.Logger != nil {
		return c.Logger
	}

	return &defaultConfig.logger
}
