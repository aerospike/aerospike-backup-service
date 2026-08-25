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
	// HTTPSServer is the backup service HTTPS server configuration.
	HTTPSServer *HTTPSServerConfig `yaml:"https,omitempty" json:"https,omitempty"`
	// Logger is the backup service logger configuration.
	Logger *LoggerConfig `yaml:"logger,omitempty" json:"logger,omitempty"`
	// Backup contains service-level backup settings.
	Backup *BackupCommonConfig `yaml:"backup,omitempty" json:"backup,omitempty"`
}

func (c *ServiceConfig) Validate(opts ValidationOptions) error {
	if err := c.HTTPServer.Validate(); err != nil {
		return fmt.Errorf("`http` validation error: %w", err)
	}
	if err := c.HTTPSServer.Validate(opts); err != nil {
		return fmt.Errorf("`https` validation error: %w", err)
	}
	if err := c.validateListeners(); err != nil {
		return err
	}
	if err := c.Logger.Validate(); err != nil {
		return fmt.Errorf("`logger` validation error: %w", err)
	}
	if err := c.Backup.Validate(); err != nil {
		return fmt.Errorf("`backup` validation error: %w", err)
	}

	return nil
}

func (c *ServiceConfig) validateListeners() error {
	httpEnabled := c.HTTPServer == nil || !c.HTTPServer.Disabled
	httpsEnabled := c.HTTPSServer != nil && !c.HTTPSServer.Disabled

	if !httpEnabled && !httpsEnabled {
		return errors.New("service.http and service.https cannot both be disabled")
	}
	if httpEnabled && httpsEnabled {
		httpPort := model.ServiceConfig{ServerHTTP: c.HTTPServer.ToModel()}.GetServerHTTPOrDefault().GetPortOrDefault()
		httpsPort := c.HTTPSServer.ToModel().GetPortOrDefault()
		if httpPort == httpsPort {
			return fmt.Errorf("service.http and service.https cannot use the same port %d", httpPort)
		}
	}

	return nil
}

func (c *ServiceConfig) ToModel() *model.ServiceConfig {
	return &model.ServiceConfig{
		ServerHTTP:  c.HTTPServer.ToModel(),
		ServerHTTPS: c.HTTPSServer.ToModel(),
		Logger:      c.Logger.ToModel(),
		Backup:      c.Backup.ToModel(),
	}
}

func (c *ServiceConfig) fromModel(m *model.ServiceConfig) {
	if m == nil {
		return
	}

	if m.ServerHTTP != nil {
		c.HTTPServer = &HTTPServerConfig{}
		c.HTTPServer.fromModel(m.ServerHTTP)
	}

	if m.ServerHTTPS != nil {
		c.HTTPSServer = &HTTPSServerConfig{}
		c.HTTPSServer.fromModel(m.ServerHTTPS)
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

	if e := c.HTTPSServer.Compare(other.HTTPSServer); e != nil {
		err = errors.Join(err, fmt.Errorf("HTTPSServer changes: %w", e))
	}

	if e := c.Logger.Compare(other.Logger); e != nil {
		err = errors.Join(err, fmt.Errorf("logger changes: %w", e))
	}

	if e := c.Backup.Compare(other.Backup); e != nil {
		err = errors.Join(err, fmt.Errorf("backup changes: %w", e))
	}

	return err
}
