package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestBackupCompletionHandler_OnFailure(t *testing.T) {
	ctrl := gomock.NewController(t)

	routine := &model.BackupRoutine{Name: "routine-1"}
	registry := NewMockBackupStateRegistry(ctrl)
	registry.EXPECT().BackupEnded("routine-1", model.BackupTypeFull)

	handler := NewBackupCompletionHandler(registry, nil, nil)
	handler.OnFailure(routine, model.BackupTypeFull)
}

// TestBackupCompletionHandler_OnSuccess_DropsTheBackupFirst checks that the backup is gone from the
// registry when OnSuccess returns, before its rescan: the orchestrator then releases the start's
// reservation, and the start rules must not see the backup as running.
func TestBackupCompletionHandler_OnSuccess_DropsTheBackupFirst(t *testing.T) {
	ctrl := gomock.NewController(t)

	routine := &model.BackupRoutine{Name: "routine-1"}
	registry := NewMockBackupStateRegistry(ctrl)
	var dropped atomic.Bool
	release := make(chan struct{})
	rescanned := make(chan struct{})
	registry.EXPECT().BackupEnded("routine-1", model.BackupTypeIncremental).Do(func(string, model.BackupType) {
		dropped.Store(true)
	})
	registry.EXPECT().BackupSucceeded(gomock.Any(), routine).Do(func(context.Context, *model.BackupRoutine) {
		<-release
		close(rescanned)
	})

	NewBackupCompletionHandler(registry, nil, nil).
		OnSuccess(t.Context(), routine, model.BackupTypeIncremental, time.Now(), slog.New(slog.DiscardHandler))

	require.True(t, dropped.Load(), "the backup is still in the registry")
	close(release)
	waitAsyncDone(t, rescanned, "history rescan")
}

func TestBackupCompletionHandler_OnSuccess_Incremental(t *testing.T) {
	ctrl := gomock.NewController(t)

	routine := &model.BackupRoutine{Name: "routine-1"}
	registry := NewMockBackupStateRegistry(ctrl)
	recorded := make(chan struct{})
	registry.EXPECT().BackupEnded("routine-1", model.BackupTypeIncremental)
	registry.EXPECT().BackupSucceeded(gomock.Any(), routine).
		Do(func(context.Context, *model.BackupRoutine) { close(recorded) })

	handler := NewBackupCompletionHandler(
		registry,
		NewMockBackupRetentionManager(ctrl),
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

	routine := &model.BackupRoutine{
		Name: "routine-1",
		BackupPolicy: &model.BackupPolicy{
			WithClusterConfig: new(true),
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

	registry.EXPECT().BackupEnded("routine-1", model.BackupTypeFull)
	registry.EXPECT().BackupSucceeded(gomock.Any(), routine).
		Do(func(context.Context, *model.BackupRoutine) { close(recorded) })
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

	handler := NewBackupCompletionHandler(registry, retention, clusterWriter)
	handler.OnSuccess(ctx, routine, model.BackupTypeFull, timestamp, slog.New(slog.DiscardHandler))

	waitAsyncDone(t, recorded, "successful full backup recorded")
	waitAsyncDone(t, retentionDone, "retention cleanup")
	waitAsyncDone(t, clusterConfigDone, "cluster config backup")
}

func TestBackupCompletionHandler_OnSuccess_FullSkipsClusterConfigWhenDisabled(t *testing.T) {
	ctrl := gomock.NewController(t)

	routine := &model.BackupRoutine{
		Name:         "routine-1",
		BackupPolicy: &model.BackupPolicy{WithClusterConfig: new(false)},
	}
	ctx := t.Context()

	registry := NewMockBackupStateRegistry(ctrl)
	retention := NewMockBackupRetentionManager(ctrl)

	recorded := make(chan struct{})
	retentionDone := make(chan struct{})

	registry.EXPECT().BackupEnded("routine-1", model.BackupTypeFull)
	registry.EXPECT().BackupSucceeded(gomock.Any(), routine).
		Do(func(context.Context, *model.BackupRoutine) { close(recorded) })
	retention.EXPECT().ApplyRetention(ctx, routine).
		DoAndReturn(func(context.Context, *model.BackupRoutine) error {
			close(retentionDone)
			return nil
		})

	handler := NewBackupCompletionHandler(registry, retention, NewMockClusterConfigWriter(ctrl))
	handler.OnSuccess(ctx, routine, model.BackupTypeFull, time.Now(), slog.New(slog.DiscardHandler))

	waitAsyncDone(t, recorded, "successful full backup recorded")
	waitAsyncDone(t, retentionDone, "retention cleanup")
}

// TestBackupCompletionHandler_OnSuccess_RetentionAfterRescan checks that retention starts only once
// the history rescan is done, so that the rescan never waits for a delete.
func TestBackupCompletionHandler_OnSuccess_RetentionAfterRescan(t *testing.T) {
	ctrl := gomock.NewController(t)

	routine := &model.BackupRoutine{Name: "routine-1", BackupPolicy: &model.BackupPolicy{}}
	ctx := t.Context()

	registry := NewMockBackupStateRegistry(ctrl)
	retention := NewMockBackupRetentionManager(ctrl)

	release := make(chan struct{})
	var started atomic.Bool
	retentionDone := make(chan struct{})
	registry.EXPECT().BackupEnded("routine-1", model.BackupTypeFull)
	registry.EXPECT().BackupSucceeded(gomock.Any(), routine).
		Do(func(context.Context, *model.BackupRoutine) { <-release })
	retention.EXPECT().ApplyRetention(ctx, routine).
		DoAndReturn(func(context.Context, *model.BackupRoutine) error {
			started.Store(true)
			close(retentionDone)
			return nil
		})

	NewBackupCompletionHandler(registry, retention, nil).
		OnSuccess(ctx, routine, model.BackupTypeFull, time.Now(), slog.New(slog.DiscardHandler))

	require.Never(t, started.Load, 100*time.Millisecond, 10*time.Millisecond,
		"retention started during the history rescan")
	close(release)
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
	registry.EXPECT().BackupEnded("routine-1", model.BackupTypeFull)
	registry.EXPECT().BackupSucceeded(gomock.Any(), routine).AnyTimes()
	retention.EXPECT().ApplyRetention(ctx, routine).Return(retentionErr)

	handler := NewBackupCompletionHandler(registry, retention, nil)
	handler.OnSuccess(ctx, routine, model.BackupTypeFull, time.Now(), logger)

	require.Eventually(t, func() bool {
		return strings.Contains(logBuf.String(), "Failed to clean up old backups")
	}, asyncWaitTimeout, 10*time.Millisecond)
}
