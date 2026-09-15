package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// TestInitComponents_MinimalConfig is a smoke test: a config with no clusters, storage, or
// routines requires no live Aerospike connection, so InitComponents can be exercised as a plain
// unit test and still wire the full object graph (scheduler + HTTP server).
func TestInitComponents_MinimalConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("service:\n"), 0o600))

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	components, err := InitComponents(ctx, configPath, false)
	require.NoError(t, err)

	require.NotNil(t, components.Scheduler)
	require.Len(t, components.Servers, 1)
	// The backup state registry, the restore job holder, the scheduler, the metrics
	// collector and the TLS provider: a component missing from this list is a component
	// that is wired but never started.
	require.Len(t, components.components, 5)

	components.Scheduler.Start(ctx)
}

// TestInitComponents_StartsNothing is the executable form of the contract in the
// InitComponents doc comment and in CLAUDE.md: components are wired but not started.
//
// Two checks, because each sees what the other cannot. goleak finds goroutines that
// outlive the call; the third-party ones the object graph inevitably brings along are
// named and ignored one at a time, so a new name in that list is a review question,
// not a test fix. The port and scheduler checks find what a goroutine count misses:
// a listener bound without a serving goroutine, a scheduler started but idle.
func TestInitComponents_StartsNothing(t *testing.T) {
	before := goleak.IgnoreCurrent()

	port := freePort(t)
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	config := fmt.Sprintf("service:\n  http:\n    address: 127.0.0.1\n    port: %d\n", port)
	require.NoError(t, os.WriteFile(configPath, []byte(config), 0o600))

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	components, err := InitComponents(ctx, configPath, false)
	require.NoError(t, err)
	require.Len(t, components.Servers, 1)

	goleak.VerifyNone(t, before,
		// lumberjack starts its rotation goroutine on the first log write and never stops it.
		goleak.IgnoreAnyFunction("gopkg.in/natefinch/lumberjack%2ev2.(*Logger).millRun"),
	)
	require.False(t, components.Scheduler.IsStarted(), "scheduler is running before Start")

	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", addr)
	require.NoError(t, err, "the HTTP listener was bound before Start")
	require.NoError(t, listener.Close())
}

// TestComponents_RunServesUntilCanceled drives the whole lifecycle through the one entry
// point main uses: Run brings the listener up, and canceling the context brings everything
// down cleanly.
func TestComponents_RunServesUntilCanceled(t *testing.T) {
	port := freePort(t)
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	config := fmt.Sprintf("service:\n  http:\n    address: 127.0.0.1\n    port: %d\n", port)
	require.NoError(t, os.WriteFile(configPath, []byte(config), 0o600))

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	components, err := InitComponents(ctx, configPath, false)
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		done <- components.Run(ctx)
	}()

	client := &http.Client{Timeout: 100 * time.Millisecond}
	require.Eventually(t, func() bool {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/health", nil)
		require.NoError(t, err)
		resp, err := client.Do(req)
		if err != nil {
			return false
		}
		_ = resp.Body.Close()

		return resp.StatusCode == http.StatusOK
	}, 5*time.Second, 10*time.Millisecond, "Run did not bring the HTTP listener up")
	require.True(t, components.Scheduler.IsStarted())

	cancel()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after its context was canceled")
	}
	// The scheduler stops itself when the context it was started with is canceled.
	require.Eventually(t, func() bool {
		return !components.Scheduler.IsStarted()
	}, time.Second, 10*time.Millisecond, "canceling the run context did not stop the scheduler")
}

// freePort returns a port that is free at the moment of the call.
func freePort(t *testing.T) int {
	t.Helper()

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())

	return port
}

func TestInitComponents_ConfigLoadError(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "does-not-exist.yaml")

	components, err := InitComponents(t.Context(), configPath, false)

	require.Error(t, err)
	require.ErrorContains(t, err, "failed to load configuration")
	require.Nil(t, components)
}

func TestInitComponents_InvalidConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "invalid.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("invalid: [yaml"), 0o600))

	components, err := InitComponents(t.Context(), configPath, false)

	require.Error(t, err)
	require.ErrorContains(t, err, "failed to load configuration")
	require.Nil(t, components)
}

func TestInitComponents_MissingTLSFiles(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	config := `service:
  https:
    cert-file: /missing/server.pem
    key-file: /missing/server-key.pem
`
	require.NoError(t, os.WriteFile(configPath, []byte(config), 0o600))

	components, err := InitComponents(t.Context(), configPath, false)

	require.ErrorContains(t, err, "failed to create HTTPS server")
	require.Nil(t, components)
}

func TestInitComponents_DisabledHTTPSDoesNotRequireTLSFiles(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	config := `service:
  https:
    disabled: true
    cert-file: /missing/server.pem
    key-file: /missing/server-key.pem
`
	require.NoError(t, os.WriteFile(configPath, []byte(config), 0o600))

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	components, err := InitComponents(ctx, configPath, false)
	require.NoError(t, err)
	require.NotNil(t, components)
}

// TestInitComponents_BuildContextIsNotALifetime is the executable form of the second half
// of the InitComponents contract: the build context is for loading only. Canceling it the
// moment the graph is built must change nothing, because the run context Start hands out
// is the only lifetime any component may have taken.
//
// Any component that captured the build context instead - a watcher, a scheduler, a job
// holder - stops here and takes the service down with it.
func TestInitComponents_BuildContextIsNotALifetime(t *testing.T) {
	port := freePort(t)
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	config := fmt.Sprintf("service:\n  http:\n    address: 127.0.0.1\n    port: %d\n", port)
	require.NoError(t, os.WriteFile(configPath, []byte(config), 0o600))

	buildCtx, cancelBuild := context.WithCancel(t.Context())
	components, err := InitComponents(buildCtx, configPath, false)
	require.NoError(t, err)

	cancelBuild() // loading is over; from here the build context means nothing.

	runCtx, cancelRun := context.WithCancel(t.Context())
	t.Cleanup(cancelRun)

	done := make(chan error, 1)
	go func() {
		done <- components.Run(runCtx)
	}()

	client := &http.Client{Timeout: 100 * time.Millisecond}
	require.Eventually(t, func() bool {
		req, err := http.NewRequestWithContext(runCtx, http.MethodGet, "http://"+addr+"/health", nil)
		require.NoError(t, err)

		resp, err := client.Do(req)
		if err != nil {
			return false
		}
		defer resp.Body.Close()

		return resp.StatusCode == http.StatusOK
	}, 2*time.Second, 10*time.Millisecond, "the service did not come up on a canceled build context")

	require.True(t, components.Scheduler.IsStarted(), "the scheduler took its lifetime from the build context")

	cancelRun()
	require.NoError(t, <-done)
}

// TestComponents_CheckStartsNothing pins step 2 of the startup sequence: Check is the
// optional validation pass between building the graph and starting it. It probes the
// configured clusters and storage and returns - it must not bring any component up, and
// with nothing configured to probe it must not fail either.
func TestComponents_CheckStartsNothing(t *testing.T) {
	before := goleak.IgnoreCurrent()

	port := freePort(t)
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	config := fmt.Sprintf(
		"service:\n  http:\n    address: 127.0.0.1\n    port: %d\nstorage:\n  local:\n    local-storage:\n      path: %s\n",
		port, t.TempDir())
	require.NoError(t, os.WriteFile(configPath, []byte(config), 0o600))

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	components, err := InitComponents(ctx, configPath, false)
	require.NoError(t, err)

	components.Check(ctx)

	goleak.VerifyNone(t, before,
		// lumberjack starts its rotation goroutine on the first log write and never stops it.
		goleak.IgnoreAnyFunction("gopkg.in/natefinch/lumberjack%2ev2.(*Logger).millRun"),
	)
	require.False(t, components.Scheduler.IsStarted(), "scheduler is running after Check")

	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", addr)
	require.NoError(t, err, "the HTTP listener was bound by Check")
	require.NoError(t, listener.Close())
}
