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

// String returns the wire value of the log level.
func (l LogLevel) String() string {
	return string(l)
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
