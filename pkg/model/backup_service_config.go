package model

// BackupServiceConfig represents the backup service configuration properties.
// @Description BackupServiceConfig represents the backup service configuration properties.
type BackupServiceConfig struct {
	// HTTPServer is the backup service HTTP server configuration.
	HTTPServer *HTTPServerConfig
	// Logger is the backup service logger configuration.
	Logger *LoggerConfig
}

// NewBackupServiceConfigWithDefaultValues returns a new BackupServiceConfig with default values.
func NewBackupServiceConfigWithDefaultValues() *BackupServiceConfig {
	return &BackupServiceConfig{
		HTTPServer: &HTTPServerConfig{},
		Logger:     &LoggerConfig{},
	}
}
