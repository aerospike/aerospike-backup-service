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

// fakeStorageReaderWriter is a hand-written test double for storageReaderWriter,
// allowing control over the exact bytes returned/captured without real storage I/O.
type fakeStorageReaderWriter struct {
	readContent []byte
	readErr     error
	writeErr    error

	writtenFileName string
	writtenContent  []byte
}

func (f *fakeStorageReaderWriter) ReadFile(_ context.Context, _ model.Storage, _ string) ([]byte, error) {
	return f.readContent, f.readErr
}

func (f *fakeStorageReaderWriter) WriteDataFile(
	_ context.Context, _ model.Storage, fileName string, content []byte,
) error {
	f.writtenFileName = fileName
	f.writtenContent = content
	return f.writeErr
}

func TestStorageManager_Read(t *testing.T) {
	tests := []struct {
		name        string
		fake        *fakeStorageReaderWriter
		expectError string
	}{
		{
			name:        "read error",
			fake:        &fakeStorageReaderWriter{readErr: errors.New("read boom")},
			expectError: "failed to read configuration from storage",
		},
		{
			name:        "invalid yaml content",
			fake:        &fakeStorageReaderWriter{readContent: []byte("invalid: [yaml")},
			expectError: "failed to unmarshal configuration",
		},
		{
			name: "success",
			fake: &fakeStorageReaderWriter{readContent: []byte("service:\n")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockNsValidator := aerospike.NewMockNamespaceValidator(ctrl)
			mockNsValidator.EXPECT().Validate(gomock.Any(), gomock.Any()).AnyTimes()

			manager := newStorageManager(&model.LocalStorage{Path: "/config.yaml"}, mockNsValidator, tt.fake)
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
		fake        *fakeStorageReaderWriter
		expectError string
	}{
		{
			name:        "write error",
			fake:        &fakeStorageReaderWriter{writeErr: errors.New("write boom")},
			expectError: "failed to write configuration to storage",
		},
		{
			name: "success",
			fake: &fakeStorageReaderWriter{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockNsValidator := aerospike.NewMockNamespaceValidator(ctrl)

			manager := newStorageManager(&model.LocalStorage{Path: "/config.yaml"}, mockNsValidator, tt.fake)
			err := manager.Write(t.Context(), model.NewConfig())

			if tt.expectError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectError)
				return
			}
			require.NoError(t, err)
			require.Contains(t, string(tt.fake.writtenContent), "yaml-language-server")
			require.Empty(t, tt.fake.writtenFileName)
		})
	}
}

func TestStorageManager_WriteThenRead_RoundTrip(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockNsValidator := aerospike.NewMockNamespaceValidator(ctrl)
	mockNsValidator.EXPECT().Validate(gomock.Any(), gomock.Any()).AnyTimes()

	fake := &fakeStorageReaderWriter{}
	manager := newStorageManager(&model.LocalStorage{Path: "/config.yaml"}, mockNsValidator, fake)

	require.NoError(t, manager.Write(t.Context(), model.NewConfig()))

	// Simulate the round trip: what was written becomes what's read back.
	fake.readContent = fake.writtenContent

	cfg, err := manager.Read(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cfg)
}
