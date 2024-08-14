package dto

import (
	"fmt"
	"slices"
	"strings"

	"github.com/aerospike/aerospike-backup-service/pkg/model"
)

// Config represents the service configuration file.
// @Description Config represents the service configuration file.
type Config struct {
	ServiceConfig     *BackupServiceConfig         `json:"service,omitempty"`
	AerospikeClusters map[string]*AerospikeCluster `json:"aerospike-clusters,omitempty"`
	Storage           map[string]*Storage          `json:"storage,omitempty"`
	BackupPolicies    map[string]*BackupPolicy     `json:"backup-policies,omitempty"`
	BackupRoutines    map[string]*BackupRoutine    `json:"backup-routines,omitempty"`
	SecretAgents      map[string]*SecretAgent      `json:"secret-agent,omitempty"`
}

func MapConfigFromDTO(dto Config) model.Config {
	c := model.Config{}
	if dto.ServiceConfig != nil {
		srvConf := MapBackupServiceConfigFromDTO(*dto.ServiceConfig)
		c.ServiceConfig = &srvConf
	}
	if dto.AerospikeClusters != nil {
		clusters := make(map[string]*model.AerospikeCluster, len(dto.AerospikeClusters))
		for k, v := range dto.AerospikeClusters {
			cluster := MapAerospikeClusterFromDTO(*v)
			clusters[k] = &cluster
		}
	}
	if dto.Storage != nil {
		storages := make(map[string]*model.Storage, len(dto.Storage))
		for k, v := range dto.Storage {
			storage := MapStorageFromDTO(*v)
			storages[k] = &storage
		}
	}
	if dto.BackupPolicies != nil {
		policies := make(map[string]*model.BackupPolicy, len(dto.BackupPolicies))
		for k, v := range dto.BackupPolicies {
			policy := MapBackupPolicyFromDTO(*v)
			policies[k] = &policy
		}
	}
	if dto.BackupRoutines != nil {
		routines := make(map[string]*model.BackupRoutine, len(dto.BackupRoutines))
		for k, v := range dto.BackupRoutines {
			routine := MapBackupRoutineFromDTO(*v)
			routines[k] = &routine
		}
	}
	if dto.SecretAgents != nil {
		sas := make(map[string]*model.SecretAgent, len(dto.SecretAgents))
		for k, v := range dto.SecretAgents {
			sa := MapSecretAgentFromDTO(*v)
			sas[k] = &sa
		}
	}

	return c
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	for name, routine := range c.BackupRoutines {
		if name == "" {
			return emptyFieldValidationError("routine name")
		}
		if err := routine.Validate(c); err != nil {
			return fmt.Errorf("backup routine '%s' validation error: %s", name, err.Error())
		}
	}

	for name, storage := range c.Storage {
		if name == "" {
			return emptyFieldValidationError("storage name")
		}
		if err := storage.Validate(); err != nil {
			return fmt.Errorf("storage '%s' validation error: %s", name, err.Error())
		}
	}

	for name, cluster := range c.AerospikeClusters {
		if name == "" {
			return emptyFieldValidationError("cluster name")
		}
		if err := cluster.Validate(); err != nil {
			return fmt.Errorf("cluster '%s' validation error: %s", name, err.Error())
		}
	}

	for name, policy := range c.BackupPolicies {
		if name == "" {
			return emptyFieldValidationError("policy name")
		}
		if err := policy.Validate(); err != nil {
			return err
		}
	}

	if err := c.ServiceConfig.HTTPServer.Validate(); err != nil {
		return err
	}

	if err := c.ServiceConfig.Logger.Validate(); err != nil {
		return err
	}

	return nil
}

// BackupServiceConfig represents the backup service configuration properties.
// @Description BackupServiceConfig represents the backup service configuration properties.
type BackupServiceConfig struct {
	// HTTPServer is the backup service HTTP server configuration.
	HTTPServer *HTTPServerConfig `json:"http,omitempty"`
	// Logger is the backup service logger configuration.
	Logger *LoggerConfig `json:"logger,omitempty"`
}

func MapBackupServiceConfigFromDTO(dto BackupServiceConfig) model.BackupServiceConfig {
	c := model.BackupServiceConfig{}
	if dto.HTTPServer != nil {
		httpConfig := MapHTTPServerConfigFromDTO(*dto.HTTPServer)
		c.HTTPServer = &httpConfig
	}
	if dto.Logger != nil {
		loggerConfig := MapLoggerConfigFromDTO(*dto.Logger)
		c.Logger = &loggerConfig
	}
	return c
}

// HTTPServerConfig represents the service's HTTP server configuration.
// @Description HTTPServerConfig represents the service's HTTP server configuration.
type HTTPServerConfig struct {
	// The address to listen on.
	Address *string `json:"address,omitempty" default:"0.0.0.0" example:"0.0.0.0"`
	// The port to listen on.
	Port *int `json:"port,omitempty" default:"8080" example:"8080"`
	// HTTP rate limiter configuration.
	Rate *RateLimiterConfig `json:"rate,omitempty"`
	// ContextPath customizes path for the API endpoints.
	ContextPath *string `json:"context-path,omitempty" default:"/"`
}

func MapHTTPServerConfigFromDTO(dto HTTPServerConfig) model.HTTPServerConfig {
	c := model.HTTPServerConfig{
		Address:     dto.Address,
		Port:        dto.Port,
		ContextPath: dto.ContextPath,
	}
	if dto.Rate != nil {
		rate := MapRateLimiterConfigFromDTO(*dto.Rate)
		c.Rate = &rate
	}
	return c
}

// Validate validates the HTTP server configuration.
func (s *HTTPServerConfig) Validate() error {
	if s.ContextPath != nil && !strings.HasPrefix(*s.ContextPath, "/") {
		return fmt.Errorf("context-path must start with a slash: %s", *s.ContextPath)
	}
	return nil
}

// RateLimiterConfig represents the service's HTTP server rate limiter configuration.
// @Description RateLimiterConfig is the HTTP server rate limiter configuration.
type RateLimiterConfig struct {
	// Rate limiter tokens per second threshold.
	Tps *int `json:"tps,omitempty" default:"1024" example:"1024"`
	// Rate limiter token bucket size (bursts threshold).
	Size *int `json:"size,omitempty" default:"1024" example:"1024"`
	// The list of ips to whitelist in rate limiting.
	WhiteList []string `json:"white-list,omitempty" default:""`
}

func MapRateLimiterConfigFromDTO(dto RateLimiterConfig) model.RateLimiterConfig {
	return model.RateLimiterConfig{
		Tps:       dto.Tps,
		Size:      dto.Size,
		WhiteList: dto.WhiteList,
	}
}

// LoggerConfig represents the backup service logger configuration.
// @Description LoggerConfig represents the backup service logger configuration.
type LoggerConfig struct {
	// Level is the logger level.
	Level *string `yaml:"level,omitempty" json:"level,omitempty" default:"DEBUG" enums:"TRACE,DEBUG,INFO,WARN,WARNING,ERROR"`
	// Format is the logger format (PLAIN, JSON).
	Format *string `yaml:"format,omitempty" json:"format,omitempty" default:"PLAIN" enums:"PLAIN,JSON"`
	// Whether to enable logging to the standard output.
	StdoutWriter *bool `yaml:"stdout-writer,omitempty" json:"stdout-writer,omitempty" default:"true"`
	// File writer logging configuration.
	FileWriter *FileLoggerConfig `yaml:"file-writer,omitempty" json:"file-writer,omitempty" default:""`
}

func MapLoggerConfigFromDTO(dto LoggerConfig) model.LoggerConfig {
	c := model.LoggerConfig{
		Level:        dto.Level,
		Format:       dto.Format,
		StdoutWriter: dto.StdoutWriter,
	}
	if dto.FileWriter != nil {
		file := MapFileLoggerConfigFromDTO(*dto.FileWriter)
		c.FileWriter = &file
	}

	return c
}

var (
	validLoggerLevels      = []string{"TRACE", "DEBUG", "INFO", "WARN", "WARNING", "ERROR"}
	supportedLoggerFormats = []string{"PLAIN", "JSON"}
)

// Validate validates the logger configuration.
func (l *LoggerConfig) Validate() error {
	if l.Level != nil && !slices.Contains(validLoggerLevels, *l.Level) {
		return fmt.Errorf("invalid logger level: %s", *l.Level)
	}
	if l.Format != nil && !slices.Contains(supportedLoggerFormats, *l.Format) {
		return fmt.Errorf("invalid logger format: %s", *l.Format)
	}
	if err := l.FileWriter.Validate(); err != nil {
		return err
	}
	return nil
}

// FileLoggerConfig represents the configuration for the file logger writer.
// @Description FileLoggerConfig represents the configuration for the file logger writer.
type FileLoggerConfig struct {
	// Filename is the file to write logs to.
	Filename string `json:"filename" example:"log.txt"`
	// MaxSize is the maximum size in megabytes of the log file before it gets rotated.
	MaxSize int `json:"maxsize" default:"100" example:"100"`
	// MaxAge is the maximum number of days to retain old log files based on the
	// timestamp encoded in their filename. The default is not to remove old log files
	// based on age.
	MaxAge int `json:"maxage" default:"0"`
	// MaxBackups is the maximum number of old log files to retain. The default
	// is to retain all old log files.
	MaxBackups int `json:"maxbackups" default:"0"`
	// Compress determines if the rotated log files should be compressed
	// using gzip. The default is not to perform compression.
	Compress bool `json:"compress" default:"false"`
}

func MapFileLoggerConfigFromDTO(dto FileLoggerConfig) model.FileLoggerConfig {
	return model.FileLoggerConfig{
		Filename:   dto.Filename,
		MaxSize:    dto.MaxSize,
		MaxAge:     dto.MaxAge,
		MaxBackups: dto.MaxBackups,
		Compress:   dto.Compress,
	}
}

// Validate validates the file logger configuration.
func (f *FileLoggerConfig) Validate() error {
	if f == nil {
		return nil
	}
	if f.Filename == "" {
		return emptyFieldValidationError("logger file")
	}
	if f.MaxSize < 0 {
		return fmt.Errorf("negative logger MaxSize: %d", f.MaxSize)
	}
	if f.MaxSize == 0 {
		f.MaxSize = 100 // set default in Mb
	}
	if f.MaxAge < 0 {
		return fmt.Errorf("negative logger MaxAge: %d", f.MaxAge)
	}
	if f.MaxBackups < 0 {
		return fmt.Errorf("negative logger MaxBackups: %d", f.MaxBackups)
	}
	return nil
}
