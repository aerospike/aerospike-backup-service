package dto

import (
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// LogLevel is the logger level.
// @Description LogLevel is the logger level.
type LogLevel string

const (
	LogLevelTrace   LogLevel = "TRACE"
	LogLevelDebug   LogLevel = "DEBUG"
	LogLevelInfo    LogLevel = "INFO"
	LogLevelWarn    LogLevel = "WARN"
	LogLevelWarning LogLevel = "WARNING"
	LogLevelError   LogLevel = "ERROR"
)

var logLevels = []LogLevel{
	LogLevelTrace,
	LogLevelDebug,
	LogLevelInfo,
	LogLevelWarn,
	LogLevelWarning,
	LogLevelError,
}

// Validate checks that the log level is supported.
func (l LogLevel) Validate() error {
	if _, ok := canonicalEnum(l, logLevels); ok {
		return nil
	}

	return errValidationInvalidValue("level", l, logLevels)
}

// ToModel converts the DTO log level to the model type.
func (l LogLevel) ToModel() model.LogLevel {
	c, _ := canonicalEnum(l, logLevels)
	return model.LogLevel(c)
}

// NewLogLevelFromModel creates a DTO log level from the model type.
func NewLogLevelFromModel(m model.LogLevel) LogLevel {
	return LogLevel(m)
}
