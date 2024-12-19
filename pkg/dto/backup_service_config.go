package dto

import (
	"errors"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
)

// BackupServiceConfig represents the backup service configuration properties.
// @Description BackupServiceConfig represents the backup service configuration properties.
type BackupServiceConfig struct {
	// HTTPServer is the backup service HTTP server configuration.
	HTTPServer *HTTPServerConfig `yaml:"http,omitempty" json:"http,omitempty"`
	// Logger is the backup service logger configuration.
	Logger *LoggerConfig `yaml:"logger,omitempty" json:"logger,omitempty"`
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

// Compare BackupServiceConfig with another and return detailed errors.
func (b *BackupServiceConfig) Compare(other BackupServiceConfig) error {
	var err error

	if e := b.HTTPServer.Compare(other.HTTPServer); e != nil {
		err = errors.Join(err, fmt.Errorf("HTTPServer changes: %w", e))
	}

	if e := b.Logger.Compare(other.Logger); e != nil {
		err = errors.Join(err, fmt.Errorf("logger changes: %w", e))
	}

	return err
}
