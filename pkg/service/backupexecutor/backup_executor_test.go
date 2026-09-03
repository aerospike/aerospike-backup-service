package backupexecutor

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestBackupExecutor_Run_GetClientError(t *testing.T) {
	ctrl := gomock.NewController(t)

	clientManager := aerospike.NewMockClientManager(ctrl)
	clientManager.EXPECT().
		GetClient(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("client unavailable"))

	executor := NewBackupExecutor(clientManager, storage.NewMockOperations(ctrl))
	routine := testBackupRoutine()

	_, err := executor.Run(
		t.Context(),
		routine,
		model.TimeBounds{},
		"test-ns",
		"/backup/path",
		nil,
		slog.Default(),
	)

	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to get backup client")
}

func TestBackupExecutor_Run_CreateWriterError(t *testing.T) {
	ctrl := gomock.NewController(t)

	client := aerospike.NewMockClient(ctrl)
	clientManager := aerospike.NewMockClientManager(ctrl)
	clientManager.EXPECT().
		GetClient(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(client, nil)
	clientManager.EXPECT().Close(client)

	operations := storage.NewMockOperations(ctrl)
	operations.EXPECT().
		CreateDirWriter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("writer failed"))

	executor := NewBackupExecutor(clientManager, operations)

	_, err := executor.Run(
		t.Context(),
		testBackupRoutine(),
		model.TimeBounds{},
		"test-ns",
		"/backup/path",
		nil,
		slog.Default(),
	)

	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to create backup writer")
}

func TestBackupExecutor_Run_BackupStartError(t *testing.T) {
	ctrl := gomock.NewController(t)

	writer := NewMockWriter(ctrl)
	client := aerospike.NewMockClient(ctrl)
	clientManager := aerospike.NewMockClientManager(ctrl)

	clientManager.EXPECT().
		GetClient(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(client, nil)
	clientManager.EXPECT().Close(client)

	client.EXPECT().
		Backup(gomock.Any(), gomock.Any(), writer, gomock.Nil()).
		Return(nil, errors.New("backup start failed"))

	operations := storage.NewMockOperations(ctrl)
	operations.EXPECT().
		CreateDirWriter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(writer, nil)

	executor := NewBackupExecutor(clientManager, operations)

	_, err := executor.Run(
		t.Context(),
		testBackupRoutine(),
		model.TimeBounds{},
		"test-ns",
		"/backup/path",
		nil,
		slog.Default(),
	)

	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to start scan backup")
}

func TestBackupExecutor_Run_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	writer := NewMockWriter(ctrl)
	client := aerospike.NewMockClient(ctrl)
	clientManager := aerospike.NewMockClientManager(ctrl)

	clientManager.EXPECT().
		GetClient(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(client, nil)

	client.EXPECT().
		Backup(gomock.Any(), gomock.Any(), writer, gomock.Nil()).
		Return(nil, nil)

	operations := storage.NewMockOperations(ctrl)
	operations.EXPECT().
		CreateDirWriter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(writer, nil)

	executor := NewBackupExecutor(clientManager, operations)

	handler, err := executor.Run(
		t.Context(),
		testBackupRoutine(),
		model.TimeBounds{},
		"test-ns",
		"/backup/path",
		nil,
		slog.Default(),
	)
	require.NoError(t, err)

	wrapped, ok := handler.(*closeOnWaitBackupHandler)
	require.True(t, ok)
	assert.Equal(t, client, wrapped.client)
}

func TestRunBackup_ConfigError(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := aerospike.NewMockClient(ctrl)

	_, err := runScanBackup(
		t.Context(),
		client,
		&model.BackupRoutine{
			BackupPolicy:     &model.BackupPolicy{},
			SourceCluster:    &model.AerospikeCluster{},
			FilterExpression: "invalid-expression",
		},
		model.TimeBounds{},
		"test-ns",
		nil,
	)

	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to make backup config")
}

func TestRunBackup_BackupStartError(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := aerospike.NewMockClient(ctrl)

	client.EXPECT().
		Backup(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Nil()).
		Return(nil, errors.New("backup failed"))

	_, err := runScanBackup(
		t.Context(),
		client,
		testBackupRoutine(),
		model.TimeBounds{},
		"test-ns",
		nil,
	)

	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to start scan backup")
}

func testBackupRoutine() *model.BackupRoutine {
	return &model.BackupRoutine{
		BackupPolicy:  &model.BackupPolicy{},
		IntervalCron:  "@daily",
		SourceCluster: &model.AerospikeCluster{},
		Storage:       &model.LocalStorage{Path: "/tmp/backups"},
	}
}
