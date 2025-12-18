package storage

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
)

type threadSafeBuffer struct {
	b bytes.Buffer
	m sync.Mutex
}

func (b *threadSafeBuffer) Write(p []byte) (n int, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Write(p)
}

func (b *threadSafeBuffer) String() string {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.String()
}

func TestGetGcpClientCleanup(t *testing.T) {
	// setup test logger
	defer func(old *slog.Logger) {
		slog.SetDefault(old)
	}(slog.Default())

	var buf threadSafeBuffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	gcpConfig := &model.GcpStorage{
		Endpoint: ts.URL,
	}

	_, err := getGcpClient(t.Context(), gcpConfig)
	assert.NoError(t, err)
	assert.NotContains(t, buf.String(), "Close GCP client")

	runtime.GC()
	time.Sleep(50 * time.Millisecond) // give some time for cleanup to run

	assert.Contains(t, buf.String(), "Close GCP client")
}
