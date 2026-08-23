package model

// ServiceConfig represents the backup service configuration properties.
type ServiceConfig struct {
	// HTTPServer is the backup service HTTP server configuration.
	HTTPServer *HTTPServerConfig
	// HTTPSServer is the backup service HTTPS server configuration.
	HTTPSServer *HTTPSServerConfig
	// Logger is the backup service logger configuration.
	Logger *LoggerConfig
	// Backup contains service-level backup settings.
	Backup *BackupCommonConfig
}

func (c ServiceConfig) GetHTTPServerOrDefault() *HTTPServerConfig {
	if c.HTTPServer != nil {
		return c.HTTPServer
	}

	return &defaultConfig.http
}

func (c ServiceConfig) GetLoggerOrDefault() *LoggerConfig {
	if c.Logger != nil {
		return c.Logger
	}

	return &defaultConfig.logger
}

// GetBackupCommonOrDefault returns the backup subsection or an empty default.
func (c ServiceConfig) GetBackupCommonOrDefault() *BackupCommonConfig {
	if c.Backup != nil {
		return c.Backup
	}

	return &BackupCommonConfig{}
}
