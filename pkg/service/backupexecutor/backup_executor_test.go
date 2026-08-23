package backupexecutor

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/io/storage/options"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type mockStorageWriter struct {
	writer backup.Writer
	err    error
	calls  int
}

func (m *mockStorageWriter) CreateDirWriter(
	_ context.Context,
	_ model.Storage,
	_ string,
	_ ...options.Opt,
) (backup.Writer, error) {
	m.calls++
	return m.writer, m.err
}

func TestBackupExecutor_Run_GetClientError(t *testing.T) {
	ctrl := gomock.NewController(t)

	clientManager := aerospike.NewMockClientManager(ctrl)
	clientManager.EXPECT().
		GetClient(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("client unavailable"))

	executor := NewBackupExecutor(clientManager, &mockStorageWriter{})
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

	executor := NewBackupExecutor(clientManager, &mockStorageWriter{
		err: errors.New("writer failed"),
	})

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

	executor := NewBackupExecutor(clientManager, &mockStorageWriter{writer: writer})

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

	operations := &mockStorageWriter{writer: writer}
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
	assert.Equal(t, 1, operations.calls)
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
