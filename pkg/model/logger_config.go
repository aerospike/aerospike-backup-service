package model

import (
	"fmt"
	"slices"
)

// LoggerConfig represents the backup service logger configuration.
// @Description LoggerConfig represents the backup service logger configuration.
//
//nolint:lll
type LoggerConfig struct {
	// Level is the logger level.
	Level string `yaml:"level,omitempty" json:"level,omitempty" default:"DEBUG" enums:"TRACE,DEBUG,INFO,WARN,WARNING,ERROR"`
	// Format is the logger format (PLAIN, JSON).
	Format string `yaml:"format,omitempty" json:"format,omitempty" default:"PLAIN" enums:"PLAIN,JSON"`
	// Whether to capture logs from the shared libraries.
	CaptureShared bool `yaml:"capture-shared,omitempty" json:"capture-shared,omitempty" default:"false"`
	// Whether to enable logging to the standard output.
	StdoutWriter bool `yaml:"stdout-writer,omitempty" json:"stdout-writer,omitempty" default:"true"`
	// File writer logging configuration.
	FileWriter *FileLoggerConfig `yaml:"file-writer,omitempty" json:"file-writer,omitempty" default:""`
}

var (
	validLoggerLevels      = []string{"TRACE", "DEBUG", "INFO", "WARN", "WARNING", "ERROR"}
	supportedLoggerFormats = []string{"PLAIN", "JSON"}
)

// Validate validates the logger configuration.
func (l *LoggerConfig) Validate() error {
	if !slices.Contains(validLoggerLevels, l.Level) {
		return fmt.Errorf("invalid logger level: %s", l.Level)
	}
	if !slices.Contains(supportedLoggerFormats, l.Format) {
		return fmt.Errorf("invalid logger format: %s", l.Format)
	}
	if err := l.FileWriter.Validate(); err != nil {
		return err
	}
	return nil
}

// NewLoggerConfigWithDefaultValues returns a new LoggerConfig with default values.
func NewLoggerConfigWithDefaultValues() *LoggerConfig {
	return &LoggerConfig{
		Level:        "DEBUG",
		Format:       "PLAIN",
		StdoutWriter: true,
	}
}

// FileLoggerConfig represents the configuration for the file logger writer.
// @Description FileLoggerConfig represents the configuration for the file logger writer.
type FileLoggerConfig struct {
	// Filename is the file to write logs to.
	Filename string `yaml:"filename" json:"filename" example:"log.txt"`
	// MaxSize is the maximum size in megabytes of the log file before it gets rotated.
	MaxSize int `yaml:"maxsize" json:"maxsize" default:"100" example:"100"`
	// MaxAge is the maximum number of days to retain old log files based on the
	// timestamp encoded in their filename. The default is not to remove old log files
	// based on age.
	MaxAge int `yaml:"maxage" json:"maxage" default:"0"`
	// MaxBackups is the maximum number of old log files to retain. The default
	// is to retain all old log files.
	MaxBackups int `yaml:"maxbackups" json:"maxbackups" default:"0"`
	// Compress determines if the rotated log files should be compressed
	// using gzip. The default is not to perform compression.
	Compress bool `yaml:"compress" json:"compress" default:"false"`
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
