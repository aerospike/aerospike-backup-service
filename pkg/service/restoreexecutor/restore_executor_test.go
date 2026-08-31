package restoreexecutor

import (
	"errors"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"github.com/aerospike/backup-go/io/storage/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestRestoreExecutor_Run_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	reader := NewMockStreamingReader(ctrl)
	restoreHandler := NewMockRestoreHandler(ctrl)

	client := aerospike.NewMockClient(ctrl)
	client.EXPECT().
		Restore(gomock.Any(), gomock.Any(), reader).
		Return(restoreHandler, nil)

	operations := storage.NewMockOperations(ctrl)
	operations.EXPECT().
		CreateDirReader(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(reader, nil)

	executor := NewRestoreExecutor(operations)

	handler, err := executor.Run(t.Context(), client, testRestoreRequest())
	require.NoError(t, err)
	require.NotNil(t, handler)
	assert.Same(t, restoreHandler, handler)
}

func TestRestoreExecutor_Run_RestoreStartError(t *testing.T) {
	ctrl := gomock.NewController(t)

	reader := NewMockStreamingReader(ctrl)
	client := aerospike.NewMockClient(ctrl)
	client.EXPECT().
		Restore(gomock.Any(), gomock.Any(), reader).
		Return(nil, errors.New("restore failed"))

	operations := storage.NewMockOperations(ctrl)
	operations.EXPECT().
		CreateDirReader(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(reader, nil)

	executor := NewRestoreExecutor(operations)

	_, err := executor.Run(t.Context(), client, testRestoreRequest())
	require.Error(t, err)
	assert.ErrorContains(t, err, "restore failed")
}

func TestRestoreExecutor_Run_EmptyStorage(t *testing.T) {
	ctrl := gomock.NewController(t)

	operations := storage.NewMockOperations(ctrl)
	operations.EXPECT().
		CreateDirReader(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, common.ErrEmptyStorage)

	executor := NewRestoreExecutor(operations)

	client := aerospike.NewMockClient(ctrl)

	_, err := executor.Run(t.Context(), client, testRestoreRequest())
	require.Error(t, err)
	require.ErrorIs(t, err, common.ErrEmptyStorage)
}

func TestRunRestore_CreateReaderError(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := aerospike.NewMockClient(ctrl)

	operations := storage.NewMockOperations(ctrl)
	operations.EXPECT().
		CreateDirReader(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("reader failed"))

	_, err := runScanRestore(t.Context(), client, testRestoreRequest(), operations)
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to create backup reader")
}

func TestRunRestore_RestoreStartError(t *testing.T) {
	ctrl := gomock.NewController(t)

	reader := NewMockStreamingReader(ctrl)
	client := aerospike.NewMockClient(ctrl)
	client.EXPECT().
		Restore(gomock.Any(), gomock.Any(), reader).
		Return(nil, errors.New("restore failed"))

	operations := storage.NewMockOperations(ctrl)
	operations.EXPECT().
		CreateDirReader(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(reader, nil)

	_, err := runScanRestore(t.Context(), client, testRestoreRequest(), operations)
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to start restore")
}

func testRestoreRequest() *model.RestoreRequest {
	return &model.RestoreRequest{
		Policy: model.RestorePolicy{
			Namespace: &model.RestoreNamespace{
				Source:      "source-ns",
				Destination: "dest-ns",
			},
		},
		SourceStorage: &model.LocalStorage{Path: "/tmp/backups"},
	}
}
