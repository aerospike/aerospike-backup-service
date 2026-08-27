package model

// ServiceConfig represents the backup service configuration properties.
type ServiceConfig struct {
	// ServerHTTP is the backup service HTTP server configuration.
	ServerHTTP *ServerConfigHTTP
	// ServerHTTPS is the backup service HTTPS server configuration.
	ServerHTTPS *ServerConfigHTTPS
	// Logger is the backup service logger configuration.
	Logger *LoggerConfig
	// Backup contains service-level backup settings.
	Backup *BackupCommonConfig
}

func (c ServiceConfig) GetServerHTTPOrDefault() *ServerConfigHTTP {
	if c.ServerHTTP != nil {
		return c.ServerHTTP
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
