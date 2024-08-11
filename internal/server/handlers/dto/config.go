package dto

// ConfigDTO represents the service configuration file.
// @Description ConfigDTO represents the service configuration file.
type ConfigDTO struct {
	ServiceConfig     *BackupServiceConfigDTO         `json:"service,omitempty"`
	AerospikeClusters map[string]*AerospikeClusterDTO `json:"aerospike-clusters,omitempty"`
	Storage           map[string]*StorageDTO          `json:"storage,omitempty"`
	BackupPolicies    map[string]*BackupPolicyDTO     `json:"backup-policies,omitempty"`
	BackupRoutines    map[string]*BackupRoutineDTO    `json:"backup-routines,omitempty"`
	SecretAgents      map[string]*SecretAgentDTO      `json:"secret-agent,omitempty"`
}

// BackupServiceConfigDTO represents the backup service configuration properties.
// @Description BackupServiceConfigDTO represents the backup service configuration properties.
type BackupServiceConfigDTO struct {
	// HTTPServer is the backup service HTTP server configuration.
	HTTPServer *HTTPServerConfigDTO `json:"http,omitempty"`
	// Logger is the backup service logger configuration.
	Logger *LoggerConfigDTO `json:"logger,omitempty"`
}

// HTTPServerConfigDTO represents the service's HTTP server configuration.
// @Description HTTPServerConfigDTO represents the service's HTTP server configuration.
type HTTPServerConfigDTO struct {
	// The address to listen on.
	Address *string `json:"address,omitempty" default:"0.0.0.0" example:"0.0.0.0"`
	// The port to listen on.
	Port *int `json:"port,omitempty" default:"8080" example:"8080"`
	// HTTP rate limiter configuration.
	Rate *RateLimiterConfigDTO `json:"rate,omitempty"`
	// ContextPath customizes path for the API endpoints.
	ContextPath *string `json:"context-path,omitempty" default:"/"`
}

// RateLimiterConfigDTO represents the service's HTTP server rate limiter configuration.
// @Description RateLimiterConfigDTO is the HTTP server rate limiter configuration.
type RateLimiterConfigDTO struct {
	// Rate limiter tokens per second threshold.
	Tps *int `json:"tps,omitempty" default:"1024" example:"1024"`
	// Rate limiter token bucket size (bursts threshold).
	Size *int `json:"size,omitempty" default:"1024" example:"1024"`
	// The list of ips to whitelist in rate limiting.
	WhiteList []string `json:"white-list,omitempty" default:""`
}

// LoggerConfigDTO represents the backup service logger configuration.
// @Description LoggerConfigDTO represents the backup service logger configuration.
type LoggerConfigDTO struct {
	// Level is the logger level.
	Level *string `yaml:"level,omitempty" json:"level,omitempty" default:"DEBUG" enums:"TRACE,DEBUG,INFO,WARN,WARNING,ERROR"`
	// Format is the logger format (PLAIN, JSON).
	Format *string `yaml:"format,omitempty" json:"format,omitempty" default:"PLAIN" enums:"PLAIN,JSON"`
	// Whether to enable logging to the standard output.
	StdoutWriter *bool `yaml:"stdout-writer,omitempty" json:"stdout-writer,omitempty" default:"true"`
	// File writer logging configuration.
	FileWriter *FileLoggerConfigDTO `yaml:"file-writer,omitempty" json:"file-writer,omitempty" default:""`
}

// FileLoggerConfigDTO represents the configuration for the file logger writer.
// @Description FileLoggerConfigDTO represents the configuration for the file logger writer.
type FileLoggerConfigDTO struct {
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
