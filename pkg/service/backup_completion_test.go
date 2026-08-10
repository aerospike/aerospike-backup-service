package service

import (
	"context"
	"errors"
	"log/slog"
	"sync"
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
	retention := NewMockRetentionManager(ctrl)
	clusterWriter := NewMockClusterConfigWriter(ctrl)

	recorded := make(chan struct{})
	registry.EXPECT().recordSuccessfulBackup(routine, model.BackupTypeIncremental).
		Do(func(*model.BackupRoutine, model.BackupType) { close(recorded) })

	handler := NewBackupCompletionHandler(registry, retention, clusterWriter)
	handler.OnSuccess(
		context.Background(),
		routine,
		model.BackupTypeIncremental,
		time.Now(),
		slog.New(slog.DiscardHandler),
	)

	<-recorded
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
	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)

	registry := NewMockRunningBackupsRegistry(ctrl)
	retention := NewMockRetentionManager(ctrl)
	clusterWriter := NewMockClusterConfigWriter(ctrl)

	var wg sync.WaitGroup
	wg.Add(3)

	registry.EXPECT().recordSuccessfulBackup(routine, model.BackupTypeFull).
		Do(func(*model.BackupRoutine, model.BackupType) { wg.Done() })
	retention.EXPECT().deleteOldBackups(ctx, routine).DoAndReturn(
		func(context.Context, *model.BackupRoutine) error {
			wg.Done()
			return nil
		},
	)
	clusterWriter.EXPECT().Write(ctx, routine, timestamp).DoAndReturn(
		func(context.Context, *model.BackupRoutine, time.Time) error {
			wg.Done()
			return nil
		},
	)

	handler := NewBackupCompletionHandler(registry, retention, clusterWriter)
	handler.OnSuccess(ctx, routine, model.BackupTypeFull, timestamp, logger)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for full backup completion side effects")
	}
}

func TestBackupCompletionHandler_OnSuccess_FullSkipsClusterConfigWhenDisabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	routine := &model.BackupRoutine{
		Name:         "routine-1",
		BackupPolicy: &model.BackupPolicy{WithClusterConfig: ptr.Of(false)},
	}
	ctx := context.Background()

	registry := NewMockRunningBackupsRegistry(ctrl)
	retention := NewMockRetentionManager(ctrl)

	var wg sync.WaitGroup
	wg.Add(2)

	registry.EXPECT().recordSuccessfulBackup(routine, model.BackupTypeFull).
		Do(func(*model.BackupRoutine, model.BackupType) { wg.Done() })
	retention.EXPECT().deleteOldBackups(ctx, routine).DoAndReturn(
		func(context.Context, *model.BackupRoutine) error {
			wg.Done()
			return nil
		},
	)

	handler := NewBackupCompletionHandler(registry, retention, NewMockClusterConfigWriter(ctrl))
	handler.OnSuccess(ctx, routine, model.BackupTypeFull, time.Now(), slog.New(slog.DiscardHandler))

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for retention cleanup")
	}
}

func TestBackupCompletionHandler_OnSuccess_LogsRetentionFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	routine := &model.BackupRoutine{
		Name:         "routine-1",
		BackupPolicy: &model.BackupPolicy{},
	}
	ctx := context.Background()
	capture := &slogCaptureHandler{}
	logger := slog.New(capture)

	registry := NewMockRunningBackupsRegistry(ctrl)
	retention := NewMockRetentionManager(ctrl)

	retentionErr := errors.New("retention failed")
	retentionDone := make(chan struct{})

	registry.EXPECT().recordSuccessfulBackup(routine, model.BackupTypeFull).AnyTimes()
	retention.EXPECT().deleteOldBackups(ctx, routine).DoAndReturn(
		func(context.Context, *model.BackupRoutine) error {
			close(retentionDone)
			return retentionErr
		},
	)

	handler := NewBackupCompletionHandler(registry, retention, nil)
	handler.OnSuccess(ctx, routine, model.BackupTypeFull, time.Now(), logger)

	<-retentionDone
	require.Eventually(t, func() bool {
		return capture.containsMessage("Failed to clean up old backups")
	}, time.Second, 10*time.Millisecond)
}

type slogCaptureHandler struct {
	mu       sync.Mutex
	messages []string
}

func (h *slogCaptureHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *slogCaptureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messages = append(h.messages, r.Message)
	return nil
}

func (h *slogCaptureHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *slogCaptureHandler) WithGroup(_ string) slog.Handler    { return h }

func (h *slogCaptureHandler) containsMessage(msg string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, m := range h.messages {
		if m == msg {
			return true
		}
	}
	return false
}
