package configuration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/require"
)

func TestFileConfigurationManager_Read(t *testing.T) {
	tempDir := t.TempDir()

	validFile := filepath.Join(tempDir, "valid.yaml")
	require.NoError(t, os.WriteFile(validFile, []byte("service:\n"), 0600))

	invalidFile := filepath.Join(tempDir, "invalid.yaml")
	require.NoError(t, os.WriteFile(invalidFile, []byte("invalid: [yaml"), 0600))

	tests := []struct {
		name        string
		filePath    string
		expectError string
	}{
		{
			name:        "missing path",
			filePath:    "",
			expectError: "configuration file path is missing",
		},
		{
			name:        "file does not exist",
			filePath:    filepath.Join(tempDir, "does-not-exist.yaml"),
			expectError: "failed to open file",
		},
		{
			name:        "invalid yaml content",
			filePath:    invalidFile,
			expectError: "failed to unmarshal configuration",
		},
		{
			name:     "success",
			filePath: validFile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := newFileConfigurationManager(tt.filePath)
			cfg, err := manager.Read(t.Context())

			if tt.expectError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectError)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, cfg)
		})
	}
}

func TestFileConfigurationManager_Read_ContextCanceled(t *testing.T) {
	manager := newFileConfigurationManager("/tmp/does-not-matter.yaml")

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := manager.Read(ctx)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}

func TestFileConfigurationManager_Write(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		filePath    string
		expectError string
	}{
		{
			name:        "missing path",
			filePath:    "",
			expectError: "configuration file path is missing",
		},
		{
			name:        "invalid directory",
			filePath:    filepath.Join(tempDir, "does-not-exist", "config.yaml"),
			expectError: "failed to open file for writing",
		},
		{
			name:     "success",
			filePath: filepath.Join(tempDir, "written.yaml"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := newFileConfigurationManager(tt.filePath)
			err := manager.Write(t.Context(), model.NewConfig())

			if tt.expectError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectError)
				return
			}
			require.NoError(t, err)

			content, readErr := os.ReadFile(tt.filePath)
			require.NoError(t, readErr)
			require.Contains(t, string(content), "yaml-language-server")
		})
	}
}

func TestFileConfigurationManager_Write_ContextCanceled(t *testing.T) {
	manager := newFileConfigurationManager(filepath.Join(t.TempDir(), "config.yaml"))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := manager.Write(ctx, model.NewConfig())
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}

func TestFileConfigurationManager_WriteThenRead_RoundTrip(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "round-trip.yaml")
	manager := newFileConfigurationManager(filePath)

	require.NoError(t, manager.Write(t.Context(), model.NewConfig()))

	cfg, err := manager.Read(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cfg)
}

// "[secret]" in the configuration file is not a password: it is what the API shows in place of
// one, and nothing stored stands behind it. Loading the file, at startup or on a reload, rejects
// it instead of authenticating with the placeholder.
func TestFileConfigurationManager_Read_RejectsRedactedPlaceholder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
aerospike-clusters:
  c1:
    seed-nodes: [{host-name: localhost, port: 3000}]
    credentials: {user: admin, password: "[secret]"}
`), 0600))

	_, err := newFileConfigurationManager(path).Read(t.Context())

	require.ErrorContains(t, err, "aerospike-clusters.c1.credentials.password: ")
	require.ErrorContains(t, err, "redaction placeholder and has no stored value")
}

// The --remote storage file is read on its own, outside the configuration it points to, so it
// gets the same check: a placeholder copied from GET /v1/config/storage is not a credential.
func TestReadStorage_RejectsRedactedPlaceholder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "storage.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
s3-storage:
  bucket: config-bucket
  s3-region: us-east-1
  access-key-id: key
  secret-access-key: "[secret]"
`), 0600))

	_, err := readStorage(t.Context(), path)

	require.ErrorContains(t, err, "s3-storage.secret-access-key: ")
	require.ErrorContains(t, err, "redaction placeholder and has no stored value")
}
