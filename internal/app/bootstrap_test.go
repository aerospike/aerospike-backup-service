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

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
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
	require.NotNil(t, components.MetricsCollector)
	require.NotNil(t, components.TLSProvider)
	require.Len(t, components.Servers, 1)

	components.Scheduler.Start(ctx)
	t.Cleanup(components.Scheduler.Stop)
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
	require.False(t, components.Scheduler.IsStarted(), "scheduler still running after Run returned")
}

// routineConfig wires one routine against local storage in dir. A routine is what makes a
// history sync meaningful: with none configured, applying the configuration has nothing to
// synchronize and would queue nothing even if it wanted to.
const routineConfig = `service:
aerospike-clusters:
  testCluster:
    seed-nodes:
      - host-name: 127.0.0.1
        port: 3000
storage:
  testStorage:
    local-storage:
      path: %s
backup-routines:
  testRoutine:
    source-cluster: testCluster
    storage: testStorage
    interval-cron: "@daily"
    namespaces:
      - testNamespace
`

// TestComponents_StartServesTheHistorySyncQueuedWhileBuilding pins the contract between
// InitComponents and Start: applying the configuration during the build queues a history
// sync, and only Start serves it.
//
// GetRoutineState blocks until a routine's first scan has completed, so if Start forgets a
// component the symptom is not a failure but a stall - every state read waits out
// getStateTimeout and then reports an empty state. Asserting that the read returns promptly
// is what catches a component that was wired and queued but never started.
func TestComponents_StartServesTheHistorySyncQueuedWhileBuilding(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	config := fmt.Sprintf(routineConfig, t.TempDir())
	require.NoError(t, os.WriteFile(configPath, []byte(config), 0o600))

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	components, err := InitComponents(ctx, configPath, false)
	require.NoError(t, err)

	components.Start(ctx)
	t.Cleanup(components.Stop)

	state := make(chan model.RoutineState, 1)
	go func() {
		state <- components.registry.GetRoutineState(&model.BackupRoutine{
			Name:     "testRoutine",
			Timezone: model.NewServiceLocation("", nil),
		})
	}()

	select {
	case <-state:
	case <-time.After(5 * time.Second):
		t.Fatal("routine state is still waiting for a history scan: Start did not serve the queued sync")
	}
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
	t.Cleanup(components.Scheduler.Stop)
}
