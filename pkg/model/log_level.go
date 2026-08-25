package model

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

// LogFormat is the logger format.
type LogFormat string

const (
	LogFormatPlain LogFormat = "PLAIN"
	LogFormatJSON  LogFormat = "JSON"
)
