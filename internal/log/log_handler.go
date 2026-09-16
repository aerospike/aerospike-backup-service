package log

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/reugn/go-quartz/logger"
	"github.com/reugn/go-quartz/quartz"
	"gopkg.in/natefinch/lumberjack.v2"
)

func init() {
	quartz.Sep = "_"
}

// NewHandler returns the application log handler with the configured level.
func NewHandler(config *model.LoggerConfig) (slog.Handler, error) {
	const addSource = true
	writer, err := logWriter(config)
	if err != nil {
		return nil, err
	}

	switch config.GetFormatOrDefault() {
	case model.LogFormatPlain:
		return slog.NewTextHandler(writer, &slog.HandlerOptions{
			Level:       config.GetLevelOrDefault().SlogLevel(),
			AddSource:   addSource,
			ReplaceAttr: handlerReplaceAttr,
		}), nil
	case model.LogFormatJSON:
		return slog.NewJSONHandler(writer, &slog.HandlerOptions{
			Level:       config.GetLevelOrDefault().SlogLevel(),
			AddSource:   addSource,
			ReplaceAttr: handlerReplaceAttr,
		}), nil
	default:
		panic("unsupported log format: " + config.GetFormatOrDefault())
	}
}

// handlerReplaceAttr applies all log attribute customizations in order.
var handlerReplaceAttr = func(groups []string, a slog.Attr) slog.Attr {
	redacted := decoder.RedactSecretsReplaceAttr()(groups, a)
	trace := traceLevelReplaceAttr(groups, redacted)

	return trace
}

func traceLevelReplaceAttr(_ []string, a slog.Attr) slog.Attr {
	if a.Key != slog.LevelKey {
		return a
	}

	level := a.Value.Any().(slog.Level)
	if level != slog.Level(logger.LevelTrace) {
		return a
	}

	a.Value = slog.StringValue("TRACE")

	return a
}

func logWriter(config *model.LoggerConfig) (io.Writer, error) {
	if config.FileWriter != nil {
		// lumberjack opens the file lazily on the first write and slog drops write errors, so a
		// misconfigured path would leave the service running with no log output at all.
		if err := probeLogFile(config.FileWriter.Filename); err != nil {
			return nil, fmt.Errorf("log file %q is not writable: %w", config.FileWriter.Filename, err)
		}

		fileWriter := &lumberjack.Logger{
			Filename:   config.FileWriter.Filename,
			MaxSize:    config.FileWriter.GetMaxSizeOrDefault(),
			MaxBackups: config.FileWriter.MaxBackups,
			MaxAge:     config.FileWriter.MaxAge,
			Compress:   config.FileWriter.Compress,
		}
		if config.GetStdoutWriterOrDefault() {
			return io.MultiWriter(fileWriter, os.Stdout), nil
		}

		return fileWriter, nil
	} else if config.GetStdoutWriterOrDefault() {
		return os.Stdout, nil
	}

	return &ignoreWriter{}, nil
}

// probeLogFile creates the log file's directory and opens the file for appending the way
// lumberjack will, so an unwritable destination fails at startup instead of silently.
func probeLogFile(filename string) error {
	if filename == "" {
		return nil // lumberjack falls back to a file in os.TempDir().
	}

	if err := os.MkdirAll(filepath.Dir(filename), 0o750); err != nil {
		return err
	}

	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}

	return f.Close()
}

type ignoreWriter struct{}

var _ io.Writer = (*ignoreWriter)(nil)

func (*ignoreWriter) Write(_ []byte) (n int, err error) {
	return 0, nil
}

// ToExitVal returns an exit value for the error.
func ToExitVal(err error) int {
	if err != nil {
		return 1
	}
	return 0
}
