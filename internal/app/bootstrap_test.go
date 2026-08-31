package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
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
	require.NotNil(t, components.CertReloader)
	require.Len(t, components.Servers, 1)

	components.Scheduler.Start(ctx)
	t.Cleanup(components.Scheduler.Stop)
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

	require.ErrorContains(t, err, "failed to validate TLS configuration")
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
