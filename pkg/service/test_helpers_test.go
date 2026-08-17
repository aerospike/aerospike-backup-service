package service

import (
	"bytes"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const asyncWaitTimeout = time.Second

func waitAsyncDone(t *testing.T, done <-chan struct{}, description string) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(asyncWaitTimeout):
		require.FailNowf(t, "timed out waiting for %s", description)
	}
}

// logBuffer is a thread-safe buffer for slog test output.
type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *logBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func newTestLogger(t *testing.T) (*slog.Logger, *logBuffer) {
	t.Helper()

	buf := &logBuffer{}
	logger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	return logger, buf
}
