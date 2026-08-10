package server

import (
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/server/handlers"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/require"
)

func newTestHTTPServer(t *testing.T) *HTTPServer {
	t.Helper()

	cfg := model.NewConfig()
	cfg.ServiceConfig.HTTPServer = &model.HTTPServerConfig{
		Address: ptr.Of("127.0.0.1"),
		Port:    ptr.Of(model.Port(0)), // let the OS choose a free port
	}

	svc := handlers.NewService(
		t.Context(),
		cfg,
		nil, nil, nil, nil, nil, nil, nil, nil,
	)

	return NewHTTPServer(t.Context(), svc)
}

func TestNewHTTPServer_StartAndShutdown(t *testing.T) {
	srv := newTestHTTPServer(t)
	require.NotNil(t, srv.server)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start()
	}()

	// Give the server a brief moment to start listening.
	time.Sleep(50 * time.Millisecond)

	require.NoError(t, srv.Shutdown())

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for server to stop")
	}
}
