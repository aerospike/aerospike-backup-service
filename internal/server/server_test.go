package server

import (
	"errors"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/server/handlers"
	servertls "github.com/aerospike/aerospike-backup-service/v3/internal/server/tlsconfig"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	secrets "github.com/aerospike/aerospike-backup-service/v3/pkg/service/secret"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/require"
)

type stubServer struct {
	start    func() error
	shutdown func() error
}

func (s stubServer) ServeHTTP(http.ResponseWriter, *http.Request) {}

func (s stubServer) Start() error {
	return s.start()
}

func (s stubServer) Shutdown() error {
	return s.shutdown()
}

func newTestServerHTTP(t *testing.T, httpCfg *model.ServerConfigHTTP) *serverHTTP {
	t.Helper()

	svc := handlers.NewService(
		t.Context(),
		model.NewConfig(),
		nil, nil, nil, nil, nil, nil, nil, nil,
		servertls.NewProber(secrets.NewResolver()),
	)

	return NewServerHTTP(t.Context(), httpCfg, svc).(*serverHTTP)
}

func waitForServerHTTPReady(t *testing.T, healthURL string) {
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

func TestNewServerHTTP_StartAndShutdown(t *testing.T) {
	srv := newTestServerHTTP(t, &model.ServerConfigHTTP{})
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

	waitForServerHTTPReady(t, "http://"+ln.Addr().String()+"/health")

	require.NoError(t, srv.Shutdown())

	select {
	case err := <-errCh:
		require.ErrorIs(t, err, http.ErrServerClosed)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for server to stop")
	}
}

func TestNewServerHTTP_ReadTimeoutClosesSilentClient(t *testing.T) {
	readTimeout := 100 * time.Millisecond
	srv := newTestServerHTTP(t, &model.ServerConfigHTTP{
		ListenerConfig: model.ListenerConfig{
			// ReadHeaderTimeout of 0 falls back to ReadTimeout in net/http.
			Timeout:     ptr.Of(time.Duration(0)),
			ReadTimeout: ptr.Of(readTimeout),
		},
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

	waitForServerHTTPReady(t, "http://"+ln.Addr().String()+"/health")

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

func TestRunDoesNotDeadlockWhenServerExitsCleanly(t *testing.T) {
	started := make(chan struct{})
	stop := make(chan struct{})

	servers := []HTTP{
		stubServer{
			start:    func() error { return nil },
			shutdown: func() error { return nil },
		},
		stubServer{
			start: func() error {
				close(started)
				<-stop
				return nil
			},
			shutdown: func() error {
				close(stop)
				return nil
			},
		},
	}

	done := make(chan error, 1)
	go func() {
		done <- Run(t.Context(), servers)
	}()

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the blocking server to start")
	}

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Run deadlocked after a listener returned nil from Start")
	}
}

func TestRunStopsEveryListenerWhenOneFails(t *testing.T) {
	stop := make(chan struct{})
	var shutdowns atomic.Int32

	servers := []HTTP{
		stubServer{
			start:    func() error { return errors.New("bind: address already in use") },
			shutdown: func() error { shutdowns.Add(1); return nil },
		},
		stubServer{
			start: func() error {
				<-stop
				return nil
			},
			shutdown: func() error {
				shutdowns.Add(1)
				close(stop)
				return nil
			},
		},
	}

	err := Run(t.Context(), servers)
	require.ErrorContains(t, err, "bind: address already in use")
	require.Equal(t, int32(2), shutdowns.Load())
}

func TestRunReportsShutdownFailure(t *testing.T) {
	servers := []HTTP{
		stubServer{
			start:    func() error { return nil },
			shutdown: func() error { return errors.New("shutdown timed out") },
		},
	}

	err := Run(t.Context(), servers)
	require.ErrorContains(t, err, "shutdown timed out")
}
