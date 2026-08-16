package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestBackupCompletionHandler_OnFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	routine := &model.BackupRoutine{Name: "routine-1"}
	registry := NewMockRunningBackupsRegistry(ctrl)
	registry.EXPECT().clearFailedBackup("routine-1", model.BackupTypeFull)

	handler := NewBackupCompletionHandler(registry, nil, nil)
	handler.OnFailure(routine, model.BackupTypeFull)
}

func TestBackupCompletionHandler_OnSuccess_Incremental(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	routine := &model.BackupRoutine{Name: "routine-1"}
	registry := NewMockRunningBackupsRegistry(ctrl)
	recorded := make(chan struct{})
	registry.EXPECT().recordSuccessfulBackup(routine, model.BackupTypeIncremental).
		Do(func(*model.BackupRoutine, model.BackupType) { close(recorded) })

	handler := NewBackupCompletionHandler(
		registry,
		NewMockRetentionManager(ctrl),
		NewMockClusterConfigWriter(ctrl),
	)
	handler.OnSuccess(
		t.Context(),
		routine,
		model.BackupTypeIncremental,
		time.Now(),
		slog.New(slog.DiscardHandler),
	)

	waitAsyncDone(t, recorded, "successful incremental backup recorded")
}

func TestBackupCompletionHandler_OnSuccess_FullRunsRetentionAndClusterConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	routine := &model.BackupRoutine{
		Name: "routine-1",
		BackupPolicy: &model.BackupPolicy{
			WithClusterConfig: ptr.Of(true),
		},
	}
	timestamp := time.Now()
	ctx := t.Context()

	registry := NewMockRunningBackupsRegistry(ctrl)
	retention := NewMockRetentionManager(ctrl)
	clusterWriter := NewMockClusterConfigWriter(ctrl)

	recorded := make(chan struct{})
	retentionDone := make(chan struct{})
	clusterConfigDone := make(chan struct{})

	registry.EXPECT().recordSuccessfulBackup(routine, model.BackupTypeFull).
		Do(func(*model.BackupRoutine, model.BackupType) { close(recorded) })
	retention.EXPECT().deleteOldBackups(ctx, routine).
		DoAndReturn(func(context.Context, *model.BackupRoutine) error {
			close(retentionDone)
			return nil
		})
	clusterWriter.EXPECT().Write(ctx, routine, timestamp).
		DoAndReturn(func(context.Context, *model.BackupRoutine, time.Time) error {
			close(clusterConfigDone)
			return nil
		})

	handler := NewBackupCompletionHandler(registry, retention, clusterWriter)
	handler.OnSuccess(ctx, routine, model.BackupTypeFull, timestamp, slog.New(slog.DiscardHandler))

	waitAsyncDone(t, recorded, "successful full backup recorded")
	waitAsyncDone(t, retentionDone, "retention cleanup")
	waitAsyncDone(t, clusterConfigDone, "cluster config backup")
}

func TestBackupCompletionHandler_OnSuccess_FullSkipsClusterConfigWhenDisabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	routine := &model.BackupRoutine{
		Name:         "routine-1",
		BackupPolicy: &model.BackupPolicy{WithClusterConfig: ptr.Of(false)},
	}
	ctx := t.Context()

	registry := NewMockRunningBackupsRegistry(ctrl)
	retention := NewMockRetentionManager(ctrl)

	recorded := make(chan struct{})
	retentionDone := make(chan struct{})

	registry.EXPECT().recordSuccessfulBackup(routine, model.BackupTypeFull).
		Do(func(*model.BackupRoutine, model.BackupType) { close(recorded) })
	retention.EXPECT().deleteOldBackups(ctx, routine).
		DoAndReturn(func(context.Context, *model.BackupRoutine) error {
			close(retentionDone)
			return nil
		})

	handler := NewBackupCompletionHandler(registry, retention, NewMockClusterConfigWriter(ctrl))
	handler.OnSuccess(ctx, routine, model.BackupTypeFull, time.Now(), slog.New(slog.DiscardHandler))

	waitAsyncDone(t, recorded, "successful full backup recorded")
	waitAsyncDone(t, retentionDone, "retention cleanup")
}

func TestBackupCompletionHandler_OnSuccess_LogsRetentionFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	routine := &model.BackupRoutine{
		Name:         "routine-1",
		BackupPolicy: &model.BackupPolicy{},
	}
	ctx := t.Context()
	logger, logBuf := newTestLogger(t)

	registry := NewMockRunningBackupsRegistry(ctrl)
	retention := NewMockRetentionManager(ctrl)

	retentionErr := errors.New("retention failed")
	registry.EXPECT().recordSuccessfulBackup(routine, model.BackupTypeFull).AnyTimes()
	retention.EXPECT().deleteOldBackups(ctx, routine).Return(retentionErr)

	handler := NewBackupCompletionHandler(registry, retention, nil)
	handler.OnSuccess(ctx, routine, model.BackupTypeFull, time.Now(), logger)

	require.Eventually(t, func() bool {
		return strings.Contains(logBuf.String(), "Failed to clean up old backups")
	}, asyncWaitTimeout, 10*time.Millisecond)
}
