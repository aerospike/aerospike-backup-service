package dto

import (
	"errors"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// ServiceConfig represents the backup service configuration properties.
// @Description ServiceConfig represents the backup service configuration properties.
type ServiceConfig struct {
	// HTTPServer is the backup service HTTP server configuration.
	HTTPServer *HTTPServerConfig `yaml:"http,omitempty" json:"http,omitempty"`
	// Logger is the backup service logger configuration.
	Logger *LoggerConfig `yaml:"logger,omitempty" json:"logger,omitempty"`
	// Backup contains service-level backup settings.
	Backup *BackupCommonConfig `yaml:"backup,omitempty" json:"backup,omitempty"`
}

func (c *ServiceConfig) Validate() error {
	if err := c.HTTPServer.Validate(); err != nil {
		return fmt.Errorf("`http` validation error: %w", err)
	}
	if err := c.Logger.Validate(); err != nil {
		return fmt.Errorf("`logger` validation error: %w", err)
	}
	if err := c.Backup.Validate(); err != nil {
		return fmt.Errorf("`backup` validation error: %w", err)
	}

	return nil
}

func (c *ServiceConfig) ToModel() *model.ServiceConfig {
	return &model.ServiceConfig{
		HTTPServer: c.HTTPServer.ToModel(),
		Logger:     c.Logger.ToModel(),
		Backup:     c.Backup.ToModel(),
	}
}

func (c *ServiceConfig) fromModel(m *model.ServiceConfig) {
	if m == nil {
		return
	}

	if m.HTTPServer != nil {
		c.HTTPServer = &HTTPServerConfig{}
		c.HTTPServer.fromModel(m.HTTPServer)
	}

	if m.Logger != nil {
		c.Logger = &LoggerConfig{}
		c.Logger.fromModel(m.Logger)
	}

	if m.Backup != nil {
		c.Backup = &BackupCommonConfig{}
		c.Backup.fromModel(m.Backup)
	}
}

// Compare ServiceConfig with another and return detailed errors.
func (c *ServiceConfig) Compare(other ServiceConfig) error {
	var err error

	if e := c.HTTPServer.Compare(other.HTTPServer); e != nil {
		err = errors.Join(err, fmt.Errorf("HTTPServer changes: %w", e))
	}

	if e := c.Logger.Compare(other.Logger); e != nil {
		err = errors.Join(err, fmt.Errorf("logger changes: %w", e))
	}

	if e := c.Backup.Compare(other.Backup); e != nil {
		err = errors.Join(err, fmt.Errorf("backup changes: %w", e))
	}

	return err
}
