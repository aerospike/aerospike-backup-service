package util

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/aerospike/backup/pkg/model"
	pkgutil "github.com/aerospike/backup/pkg/util"
	"gopkg.in/natefinch/lumberjack.v2"
)

// LogHandler returns the application log handler with the
// configured level.
func LogHandler(config *model.LoggerConfig) slog.Handler {
	addSource := true
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

func logWriter(config *model.LoggerConfig) io.Writer {
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
		return pkgutil.LevelTrace
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

// Check panics if the error is not nil.
func Check(e error) {
	if e != nil {
		panic(e)
	}
}

// Returns an exit value for the error.
func ToExitVal(err error) int {
	if err != nil {
		return 1
	}
	return 0
}

// CaptureStdout returns the stdout output written during the
// given function execution.
func CaptureStdout(f func()) string {
	old := os.Stdout // keep backup of the real stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f() // run the function

	outC := make(chan string)
	// copy the output in a separate goroutine so printing
	// can't block indefinitely
	go func() {
		var buf bytes.Buffer
		_, err := io.Copy(&buf, r)
		Check(err)
		outC <- buf.String()
	}()

	// back to normal state
	w.Close()
	os.Stdout = old // restoring the real stdout
	out := <-outC
	return out
}
