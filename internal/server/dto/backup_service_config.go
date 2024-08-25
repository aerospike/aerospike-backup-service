package dto

import "github.com/aerospike/aerospike-backup-service/pkg/model"

// BackupServiceConfig represents the backup service configuration properties.
// @Description BackupServiceConfig represents the backup service configuration properties.
type BackupServiceConfig struct {
	// HTTPServer is the backup service HTTP server configuration.
	HTTPServer *HTTPServerConfig `yaml:"http,omitempty" json:"http,omitempty"`
	// Logger is the backup service logger configuration.
	Logger *LoggerConfig `yaml:"logger,omitempty" json:"logger,omitempty"`
}

// NewBackupServiceConfigWithDefaultValues returns a new BackupServiceConfig with default values.
func NewBackupServiceConfigWithDefaultValues() BackupServiceConfig {
	return BackupServiceConfig{
		HTTPServer: &HTTPServerConfig{},
		Logger:     &LoggerConfig{},
	}
}

func (b *BackupServiceConfig) ToModel() *model.BackupServiceConfig {
	return &model.BackupServiceConfig{
		HTTPServer: b.HTTPServer.ToModel(),
		Logger:     b.Logger.ToModel(),
	}
}

func (b *BackupServiceConfig) fromModel(m *model.BackupServiceConfig) {
	if m == nil {
		return
	}
	if m.HTTPServer != nil {
		b.HTTPServer = &HTTPServerConfig{}
		b.HTTPServer.fromModel(m.HTTPServer)
	}

	if m.Logger != nil {
		b.Logger = &LoggerConfig{}
		b.Logger.fromModel(m.Logger)
	}
}
