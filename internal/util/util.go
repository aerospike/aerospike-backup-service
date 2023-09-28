package util

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"log/slog"
)

// LogHandler returns the application log handler with the
// configured level.
func LogHandler(level string) slog.Handler {
	return slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level:     logLevel(level),
		AddSource: true,
	})
}

// logLevel returns a level for the given string name.
// Panics on an invalid argument.
func logLevel(level string) slog.Level {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		panic(fmt.Sprintf("invalid log level configuration: %s", level))
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
