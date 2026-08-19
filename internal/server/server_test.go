package server

import (
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/server/handlers"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/require"
)

func newTestHTTPServer(t *testing.T, httpCfg *model.HTTPServerConfig) *httpServer {
	t.Helper()

	if httpCfg == nil {
		httpCfg = &model.HTTPServerConfig{}
	}
	if httpCfg.Address == "" {
		httpCfg.Address = "127.0.0.1"
	}
	if httpCfg.Port == nil {
		httpCfg.Port = ptr.Of(model.Port(0)) // let the OS choose a free port
	}

	svc := handlers.NewService(
		t.Context(),
		model.NewConfig(),
		nil, nil, nil, nil, nil, nil, nil, nil,
	)

	return NewHTTPServer(t.Context(), httpCfg, svc).(*httpServer)
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
	srv := newTestHTTPServer(t, nil)
	require.NotNil(t, srv.Server)
	require.Equal(t, 5*time.Second, srv.ReadHeaderTimeout)
	require.Equal(t, 30*time.Second, srv.ReadTimeout)
	require.Equal(t, 60*time.Second, srv.WriteTimeout)
	require.Equal(t, 120*time.Second, srv.IdleTimeout)

	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ln)
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

func TestNewHTTPServer_ReadTimeoutClosesSilentClient(t *testing.T) {
	readTimeout := 100 * time.Millisecond
	srv := newTestHTTPServer(t, &model.HTTPServerConfig{
		// ReadHeaderTimeout of 0 falls back to ReadTimeout in net/http.
		Timeout:     ptr.Of(time.Duration(0)),
		ReadTimeout: ptr.Of(readTimeout),
	})
	require.Equal(t, time.Duration(0), srv.ReadHeaderTimeout)
	require.Equal(t, readTimeout, srv.ReadTimeout)

	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ln)
	}()
	t.Cleanup(func() {
		_ = srv.Shutdown()
		<-errCh
	})

	waitForHTTPServerReady(t, "http://"+ln.Addr().String()+"/health")

	conn, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", ln.Addr().String())
	require.NoError(t, err)
	defer conn.Close()

	// Silent client: open a connection but never send a request.
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1)
	start := time.Now()
	_, err = conn.Read(buf)
	elapsed := time.Since(start)

	require.Error(t, err)
	require.ErrorIs(t, err, io.EOF)
	require.Less(t, elapsed, time.Second, "server should close the connection near ReadTimeout")
	require.GreaterOrEqual(t, elapsed, readTimeout/2)
}
