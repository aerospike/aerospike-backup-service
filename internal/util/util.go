package util

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/aerospike/aerospike-backup-service/internal/server/dto"
	"gopkg.in/natefinch/lumberjack.v2"
)

// LogHandler returns the application log handler with the
// configured level.
func LogHandler(config *dto.LoggerConfig) slog.Handler {
	const addSource = true
	writer := logWriter(config)
	switch strings.ToUpper(config.GetFormatOrDefault()) {
	case "PLAIN":
		return slog.NewTextHandler(writer, &slog.HandlerOptions{
			Level:     logLevel(config.GetLevelOrDefault()),
			AddSource: addSource,
		})
	case "JSON":
		return slog.NewJSONHandler(writer, &slog.HandlerOptions{
			Level:     logLevel(config.GetLevelOrDefault()),
			AddSource: addSource,
		})
	default:
		panic(fmt.Sprintf("unsupported log format: %s", *config.Format))
	}
}

func logWriter(config *dto.LoggerConfig) io.Writer {
	if config.FileWriter != nil {
		fileWriter := &lumberjack.Logger{
			Filename:   config.FileWriter.Filename,
			MaxSize:    config.FileWriter.MaxSize,
			MaxBackups: config.FileWriter.MaxBackups,
			MaxAge:     config.FileWriter.MaxAge,
			Compress:   config.FileWriter.Compress,
		}
		if config.GetStdoutWriterOrDefault() {
			return io.MultiWriter(fileWriter, os.Stdout)
		}
		return fileWriter
	} else if config.GetStdoutWriterOrDefault() {
		return os.Stdout
	}
	return &ignoreWriter{}
}

type ignoreWriter struct{}

var _ io.Writer = (*ignoreWriter)(nil)

func (*ignoreWriter) Write(_ []byte) (n int, err error) {
	return 0, nil
}

// logLevel returns a level for the given string name.
// Panics on an invalid argument.
func logLevel(level string) slog.Level {
	switch strings.ToUpper(level) {
	case "TRACE":
		return slog.LevelDebug
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		panic(fmt.Sprintf("invalid log level: %s", level))
	}
}

// ToExitVal returns an exit value for the error.
func ToExitVal(err error) int {
	if err != nil {
		return 1
	}
	return 0
}
