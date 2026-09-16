package model

import (
	"log/slog"

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
	switch l {
	case LogLevelTrace:
		return slog.Level(logger.LevelTrace)
	case LogLevelDebug:
		return slog.LevelDebug
	case LogLevelInfo:
		return slog.LevelInfo
	case LogLevelWarn, LogLevelWarning:
		return slog.LevelWarn
	case LogLevelError:
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
