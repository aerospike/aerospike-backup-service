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

// A run that failed leaves nothing restorable behind: the whole timestamp folder goes, with the
// data of the namespaces that did succeed and the cluster configuration if one was taken.
func TestBackupCompletionHandler_OnFailure(t *testing.T) {
	ctrl := gomock.NewController(t)

	routine := &model.BackupRoutine{Name: "routine-1"}
	registry := NewMockBackupStateRegistry(ctrl)
	registry.EXPECT().BackupFailed("routine-1", model.BackupTypeFull)

	writer := NewMockBackupWriter(ctrl)
	writer.EXPECT().Delete(gomock.Any(), routine, "routine-1/backup/1700000000000").Return(nil)

	handler := NewBackupCompletionHandler(
		registry, NewMockBackupRetentionManager(ctrl), NewMockClusterConfigWriter(ctrl),
		writer, NewPathService(nil),
	)
	handler.OnFailure(
		t.Context(), routine, model.BackupTypeFull,
		time.UnixMilli(1700000000000), slog.New(slog.DiscardHandler),
	)
}

// The cleanup outlives the run's context, so a canceled backup is cleaned up too.
func TestBackupCompletionHandler_OnFailure_CleansUpAfterCancellation(t *testing.T) {
	ctrl := gomock.NewController(t)

	routine := &model.BackupRoutine{Name: "routine-1"}
	registry := NewMockBackupStateRegistry(ctrl)
	registry.EXPECT().BackupFailed("routine-1", model.BackupTypeIncremental)

	writer := NewMockBackupWriter(ctrl)
	writer.EXPECT().Delete(gomock.Any(), routine, "routine-1/incremental/1700000000000").
		DoAndReturn(func(ctx context.Context, _ *model.BackupRoutine, _ string) error {
			require.NoError(t, ctx.Err(), "cleanup must not run on a canceled context")

			return nil
		})

	canceled, cancel := context.WithCancel(t.Context())
	cancel()

	handler := NewBackupCompletionHandler(
		registry, NewMockBackupRetentionManager(ctrl), NewMockClusterConfigWriter(ctrl),
		writer, NewPathService(nil),
	)
	handler.OnFailure(
		canceled, routine, model.BackupTypeIncremental,
		time.UnixMilli(1700000000000), slog.New(slog.DiscardHandler),
	)
}

func TestBackupCompletionHandler_OnSuccess_Incremental(t *testing.T) {
	ctrl := gomock.NewController(t)

	routine := &model.BackupRoutine{Name: "routine-1"}
	registry := NewMockBackupStateRegistry(ctrl)
	recorded := make(chan struct{})
	registry.EXPECT().BackupSucceeded(gomock.Any(), routine, model.BackupTypeIncremental).
		Do(func(context.Context, *model.BackupRoutine, model.BackupType) { close(recorded) })

	handler := NewBackupCompletionHandler(
		registry,
		NewMockBackupRetentionManager(ctrl),
		NewMockClusterConfigWriter(ctrl),
		NewMockBackupWriter(ctrl),
		NewPathService(nil),
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

	routine := &model.BackupRoutine{
		Name: "routine-1",
		BackupPolicy: &model.BackupPolicy{
			WithClusterConfig: ptr.Of(true),
		},
	}
	timestamp := time.Now()
	ctx := t.Context()

	registry := NewMockBackupStateRegistry(ctrl)
	retention := NewMockBackupRetentionManager(ctrl)
	clusterWriter := NewMockClusterConfigWriter(ctrl)

	recorded := make(chan struct{})
	retentionDone := make(chan struct{})
	clusterConfigDone := make(chan struct{})

	registry.EXPECT().BackupSucceeded(gomock.Any(), routine, model.BackupTypeFull).
		Do(func(context.Context, *model.BackupRoutine, model.BackupType) { close(recorded) })
	retention.EXPECT().ApplyRetention(ctx, routine).
		DoAndReturn(func(context.Context, *model.BackupRoutine) error {
			close(retentionDone)
			return nil
		})
	clusterWriter.EXPECT().Write(ctx, routine, timestamp).
		DoAndReturn(func(context.Context, *model.BackupRoutine, time.Time) error {
			close(clusterConfigDone)
			return nil
		})

	handler := NewBackupCompletionHandler(
		registry, retention, clusterWriter, NewMockBackupWriter(ctrl), NewPathService(nil),
	)
	handler.OnSuccess(ctx, routine, model.BackupTypeFull, timestamp, slog.New(slog.DiscardHandler))

	waitAsyncDone(t, recorded, "successful full backup recorded")
	waitAsyncDone(t, retentionDone, "retention cleanup")
	waitAsyncDone(t, clusterConfigDone, "cluster config backup")
}

func TestBackupCompletionHandler_OnSuccess_FullSkipsClusterConfigWhenDisabled(t *testing.T) {
	ctrl := gomock.NewController(t)

	routine := &model.BackupRoutine{
		Name:         "routine-1",
		BackupPolicy: &model.BackupPolicy{WithClusterConfig: ptr.Of(false)},
	}
	ctx := t.Context()

	registry := NewMockBackupStateRegistry(ctrl)
	retention := NewMockBackupRetentionManager(ctrl)

	recorded := make(chan struct{})
	retentionDone := make(chan struct{})

	registry.EXPECT().BackupSucceeded(gomock.Any(), routine, model.BackupTypeFull).
		Do(func(context.Context, *model.BackupRoutine, model.BackupType) { close(recorded) })
	retention.EXPECT().ApplyRetention(ctx, routine).
		DoAndReturn(func(context.Context, *model.BackupRoutine) error {
			close(retentionDone)
			return nil
		})

	handler := NewBackupCompletionHandler(
		registry, retention, NewMockClusterConfigWriter(ctrl), NewMockBackupWriter(ctrl), NewPathService(nil),
	)
	handler.OnSuccess(ctx, routine, model.BackupTypeFull, time.Now(), slog.New(slog.DiscardHandler))

	waitAsyncDone(t, recorded, "successful full backup recorded")
	waitAsyncDone(t, retentionDone, "retention cleanup")
}

func TestBackupCompletionHandler_OnSuccess_LogsRetentionFailure(t *testing.T) {
	ctrl := gomock.NewController(t)

	routine := &model.BackupRoutine{
		Name:         "routine-1",
		BackupPolicy: &model.BackupPolicy{},
	}
	ctx := t.Context()
	logger, logBuf := newTestLogger(t)

	registry := NewMockBackupStateRegistry(ctrl)
	retention := NewMockBackupRetentionManager(ctrl)

	retentionErr := errors.New("retention failed")
	registry.EXPECT().BackupSucceeded(gomock.Any(), routine, model.BackupTypeFull).AnyTimes()
	retention.EXPECT().ApplyRetention(ctx, routine).Return(retentionErr)

	handler := NewBackupCompletionHandler(registry, retention, nil, NewMockBackupWriter(ctrl), NewPathService(nil))
	handler.OnSuccess(ctx, routine, model.BackupTypeFull, time.Now(), logger)

	require.Eventually(t, func() bool {
		return strings.Contains(logBuf.String(), "Failed to clean up old backups")
	}, asyncWaitTimeout, 10*time.Millisecond)
}
