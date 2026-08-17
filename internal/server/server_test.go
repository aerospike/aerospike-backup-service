package server

import (
	"net"
	"net/http"
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
		Address: "127.0.0.1",
		Port:    ptr.Of(model.Port(0)), // let the OS choose a free port
	}

	svc := handlers.NewService(
		t.Context(),
		cfg,
		nil, nil, nil, nil, nil, nil, nil, nil,
	)

	return NewHTTPServer(t.Context(), svc)
}

func waitForHTTPServerReady(t *testing.T, healthURL string) {
	t.Helper()

	client := &http.Client{Timeout: 100 * time.Millisecond}
	deadline := time.Now().Add(5 * time.Second)

	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, healthURL, nil)
		require.NoError(t, err)

		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for server to start")
}

func TestNewHTTPServer_StartAndShutdown(t *testing.T) {
	srv := newTestHTTPServer(t)
	require.NotNil(t, srv.server)

	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.server.Serve(ln)
	}()

	waitForHTTPServerReady(t, "http://"+ln.Addr().String()+"/health")

	require.NoError(t, srv.Shutdown())

	select {
	case err := <-errCh:
		require.ErrorIs(t, err, http.ErrServerClosed)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for server to stop")
	}
}
