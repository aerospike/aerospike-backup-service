package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/require"
)

func TestNamespaceBackupRunnerImpl_DeleteFolder_Success(t *testing.T) {
	mocks, runner := initMocks(t)
	defer mocks.ctrl.Finish()

	impl, ok := runner.(*NamespaceBackupRunnerImpl)
	require.True(t, ok)

	routine := &model.BackupRoutine{Name: routineName}
	mocks.backendService.EXPECT().Delete(context.Background(), routine, "some/path").Return(nil)

	impl.deleteFolder(context.Background(), routine, "some/path", slog.New(slog.DiscardHandler))
}

func TestNamespaceBackupRunnerImpl_DeleteFolder_ContextCanceled(t *testing.T) {
	mocks, runner := initMocks(t)
	defer mocks.ctrl.Finish()

	impl, ok := runner.(*NamespaceBackupRunnerImpl)
	require.True(t, ok)

	routine := &model.BackupRoutine{Name: routineName}
	capture := &slogCaptureHandler{}
	logger := slog.New(capture)

	mocks.backendService.EXPECT().Delete(context.Background(), routine, "some/path").Return(context.Canceled)

	impl.deleteFolder(context.Background(), routine, "some/path", logger)

	require.True(t, capture.containsMessage("Delete folder context canceled"))
}

func TestNamespaceBackupRunnerImpl_DeleteFolder_Error(t *testing.T) {
	mocks, runner := initMocks(t)
	defer mocks.ctrl.Finish()

	impl, ok := runner.(*NamespaceBackupRunnerImpl)
	require.True(t, ok)

	routine := &model.BackupRoutine{Name: routineName}
	capture := &slogCaptureHandler{}
	logger := slog.New(capture)
	deleteErr := errors.New("delete failed")

	mocks.backendService.EXPECT().Delete(context.Background(), routine, "some/path").Return(deleteErr)

	impl.deleteFolder(context.Background(), routine, "some/path", logger)

	require.True(t, capture.containsMessage("Failed to delete folder"))
}
