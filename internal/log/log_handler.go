package log

import (
	"io"
	"log/slog"
	"os"

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
func NewHandler(config *model.LoggerConfig) slog.Handler {
	const addSource = true
	writer := logWriter(config)
	switch config.GetFormatOrDefault() {
	case model.LogFormatPlain:
		return slog.NewTextHandler(writer, &slog.HandlerOptions{
			Level:       config.GetLevelOrDefault().SlogLevel(),
			AddSource:   addSource,
			ReplaceAttr: handlerReplaceAttr,
		})
	case model.LogFormatJSON:
		return slog.NewJSONHandler(writer, &slog.HandlerOptions{
			Level:       config.GetLevelOrDefault().SlogLevel(),
			AddSource:   addSource,
			ReplaceAttr: handlerReplaceAttr,
		})
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

func logWriter(config *model.LoggerConfig) io.Writer {
	if config.FileWriter != nil {
		fileWriter := &lumberjack.Logger{
			Filename:   config.FileWriter.Filename,
			MaxSize:    config.FileWriter.GetMaxSizeOrDefault(),
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

// ToExitVal returns an exit value for the error.
func ToExitVal(err error) int {
	if err != nil {
		return 1
	}
	return 0
}
