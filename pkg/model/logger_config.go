package model

import (
	"fmt"
	"slices"
)

// LoggerConfig represents the backup service logger configuration.
// @Description LoggerConfig represents the backup service logger configuration.
type LoggerConfig struct {
	// Level is the logger level.
	Level string `yaml:"level,omitempty" json:"level,omitempty" default:"DEBUG"`
	// Format is the logger format (PLAIN, JSON).
	Format string `yaml:"format,omitempty" json:"format,omitempty" default:"PLAIN"`
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
	return nil
}

// NewLoggerConfigWithDefaultValues returns a new LoggerConfig with default values.
func NewLoggerConfigWithDefaultValues() *LoggerConfig {
	return &LoggerConfig{
		Level:  "DEBUG",
		Format: "PLAIN",
	}
}
