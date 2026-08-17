package configuration

import (
	"context"
	"errors"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestStorageManager_Read(t *testing.T) {
	tests := []struct {
		name        string
		content     []byte
		readErr     error
		expectError string
	}{
		{
			name:    "success",
			content: []byte("service:\n"),
		},
		{
			name:        "read error",
			readErr:     errors.New("read boom"),
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
			ctrl := gomock.NewController(t)

			mockOps := NewmockStorageReaderWriter(ctrl)
			mockOps.EXPECT().ReadFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(tt.content, tt.readErr)

			manager := newTestStorageManager(ctrl, mockOps)
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
	tests := []struct {
		name        string
		writeErr    error
		expectError string
	}{
		{
			name: "success",
		},
		{
			name:        "write error",
			writeErr:    errors.New("write boom"),
			expectError: "failed to write configuration to storage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			var written []byte
			mockOps := NewmockStorageReaderWriter(ctrl)
			call := mockOps.EXPECT().WriteDataFile(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
			if tt.writeErr != nil {
				call.Return(tt.writeErr)
			} else {
				call.Do(func(_ context.Context, _ model.Storage, _ string, content []byte) { written = content }).Return(nil)
			}

			manager := newTestStorageManager(ctrl, mockOps)
			err := manager.Write(t.Context(), model.NewConfig())

			if tt.expectError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectError)
				return
			}
			require.NoError(t, err)
			require.Contains(t, string(written), "yaml-language-server")
		})
	}
}

func TestStorageManager_WriteThenRead_RoundTrip(t *testing.T) {
	ctrl := gomock.NewController(t)

	var stored []byte

	mockOps := NewmockStorageReaderWriter(ctrl)
	mockOps.EXPECT().WriteDataFile(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Do(func(_ context.Context, _ model.Storage, _ string, content []byte) { stored = content }).Return(nil)
	mockOps.EXPECT().ReadFile(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ model.Storage, _ string) ([]byte, error) {
			return stored, nil
		})
	manager := newTestStorageManager(ctrl, mockOps)

	require.NoError(t, manager.Write(t.Context(), model.NewConfig()))

	cfg, err := manager.Read(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cfg)
}

func newTestStorageManager(ctrl *gomock.Controller, ops storageReaderWriter) Manager {
	mockNsValidator := aerospike.NewMockNamespaceValidator(ctrl)
	mockNsValidator.EXPECT().Validate(gomock.Any(), gomock.Any()).AnyTimes()

	return newStorageManager(&model.LocalStorage{Path: "/config.yaml"}, mockNsValidator, ops)
}
