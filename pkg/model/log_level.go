package model

import (
	"log/slog"
	"strings"

	"github.com/reugn/go-quartz/logger"
)

// LogLevel is the logger level.
type LogLevel string

const (
	LogLevelTrace   LogLevel = "TRACE"
	LogLevelDebug   LogLevel = "DEBUG"
	LogLevelInfo    LogLevel = "INFO"
	LogLevelWarn    LogLevel = "WARN"
	LogLevelWarning LogLevel = "WARNING"
	LogLevelError   LogLevel = "ERROR"
)

// String returns the wire value of the log level.
func (l LogLevel) String() string {
	return string(l)
}

// SlogLevel returns the slog level for the configured log level.
// Panics on an invalid value.
func (l LogLevel) SlogLevel() slog.Level {
	switch strings.ToUpper(l.String()) {
	case string(LogLevelTrace):
		return slog.Level(logger.LevelTrace)
	case string(LogLevelDebug):
		return slog.LevelDebug
	case string(LogLevelInfo):
		return slog.LevelInfo
	case string(LogLevelWarn), string(LogLevelWarning):
		return slog.LevelWarn
	case string(LogLevelError):
		return slog.LevelError
	default:
		panic("invalid log level: " + l.String())
	}
}

// LogFormat is the logger format.
type LogFormat string

const (
	LogFormatPlain LogFormat = "PLAIN"
	LogFormatJSON  LogFormat = "JSON"
)

// String returns the wire value of the log format.
func (f LogFormat) String() string {
	return string(f)
}
