package configuration

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestStorageManager_Read(t *testing.T) {
	tests := []struct {
		name        string
		content     []byte
		missingFile bool
		expectError string
	}{
		{
			name:    "success",
			content: []byte("service:\n"),
		},
		{
			name:        "read error",
			missingFile: true,
			expectError: "failed to read configuration from storage",
		},
		{
			name:        "invalid yaml content",
			content:     []byte("invalid: [yaml"),
			expectError: "failed to unmarshal configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, ops, configStorage := newTestStorageManager(t)

			if !tt.missingFile {
				require.NoError(t, ops.WriteDataFile(t.Context(), configStorage, "", tt.content))
			}

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

func TestStorageManager_Write(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		manager, ops, configStorage := newTestStorageManager(t)

		require.NoError(t, manager.Write(t.Context(), model.NewConfig()))

		written, err := ops.ReadFile(t.Context(), configStorage, "")
		require.NoError(t, err)
		require.Contains(t, string(written), "yaml-language-server")
	})

	t.Run("write error", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		ops := storage.NewMockOperations(ctrl)
		ops.EXPECT().
			WriteDataFile(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(errors.New("disk full"))

		manager := newStorageManager(
			&model.LocalStorage{Path: filepath.Join(t.TempDir(), "config.yml")},
			ops,
		)

		err := manager.Write(t.Context(), model.NewConfig())
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to write configuration to storage")
	})
}

func TestStorageManager_WriteThenRead_RoundTrip(t *testing.T) {
	manager, _, _ := newTestStorageManager(t)

	require.NoError(t, manager.Write(t.Context(), model.NewConfig()))

	cfg, err := manager.Read(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cfg)
}

func newTestStorageManager(t *testing.T) (Manager, storage.Operations, *model.LocalStorage) {
	t.Helper()

	configStorage := &model.LocalStorage{Path: filepath.Join(t.TempDir(), "config.yml")}
	ops := storage.NewOperations(storage.NewLocalStorageAccessor())
	return newStorageManager(configStorage, ops), ops, configStorage
}
