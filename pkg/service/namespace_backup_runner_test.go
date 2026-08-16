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
	mocks.backendService.EXPECT().Delete(t.Context(), routine, "some/path").Return(nil)

	impl.deleteFolder(t.Context(), routine, "some/path", slog.New(slog.DiscardHandler))
}

func TestNamespaceBackupRunnerImpl_DeleteFolder_ContextCanceled(t *testing.T) {
	mocks, runner := initMocks(t)
	defer mocks.ctrl.Finish()

	impl, ok := runner.(*NamespaceBackupRunnerImpl)
	require.True(t, ok)

	routine := &model.BackupRoutine{Name: routineName}
	logger, logBuf := newTestLogger(t)

	mocks.backendService.EXPECT().Delete(t.Context(), routine, "some/path").Return(context.Canceled)

	impl.deleteFolder(t.Context(), routine, "some/path", logger)

	require.Contains(t, logBuf.String(), "Delete folder context canceled")
}

func TestNamespaceBackupRunnerImpl_DeleteFolder_Error(t *testing.T) {
	mocks, runner := initMocks(t)
	defer mocks.ctrl.Finish()

	impl, ok := runner.(*NamespaceBackupRunnerImpl)
	require.True(t, ok)

	routine := &model.BackupRoutine{Name: routineName}
	logger, logBuf := newTestLogger(t)
	deleteErr := errors.New("delete failed")

	mocks.backendService.EXPECT().Delete(t.Context(), routine, "some/path").Return(deleteErr)

	impl.deleteFolder(t.Context(), routine, "some/path", logger)

	require.Contains(t, logBuf.String(), "Failed to delete folder")
}
