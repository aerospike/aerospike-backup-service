package service

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/backupexecutor"
	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var retry = models.RetryPolicy{
	MaxRetries:  2,
	BaseTimeout: time.Millisecond,
	Multiplier:  1,
}

func TestStartRetryableBackup_SuccessfulFirstAttempt(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockHandler := backupexecutor.NewMockBackupHandler(ctrl)
	stats := models.NewBackupStats()
	mockHandler.EXPECT().Wait(gomock.Any()).Return(nil)
	mockHandler.EXPECT().GetStats().Return(stats)

	successCount := 0
	failureCount := 0
	retryCount := 0

	start := func(_ context.Context) (backupexecutor.BackupHandler, error) {
		return mockHandler, nil
	}

	onFail := func(_ context.Context) {
		failureCount++
	}

	onSuccess := func(_ context.Context, _ *models.BackupStats) error {
		successCount++
		return nil
	}

	onRetry := func() {
		retryCount++
	}

	handler := newRetryableBackupHandler(t.Context(), retry, retryableBackupCallbacks{
		Start: start, OnFail: onFail, OnSuccess: onSuccess, OnRetry: onRetry,
	}, slog.Default())
	err := handler.Wait(t.Context())

	require.NoError(t, err)
	assert.Equal(t, 0, failureCount)
	assert.Equal(t, 1, successCount)
	assert.Equal(t, 0, retryCount)
}

func TestStartRetryableBackup_WaitFailsThenSucceeds(t *testing.T) {
	ctrl := gomock.NewController(t)
	failedHandler := backupexecutor.NewMockBackupHandler(ctrl)
	successHandler := backupexecutor.NewMockBackupHandler(ctrl)
	stats := models.NewBackupStats()

	failedHandler.EXPECT().Wait(gomock.Any()).Return(errors.New("wait failed"))
	successHandler.EXPECT().Wait(gomock.Any()).Return(nil)
	successHandler.EXPECT().GetStats().Return(stats)

	attemptCount := 0
	successCount := 0
	failureCount := 0
	retryCount := 0

	start := func(_ context.Context) (backupexecutor.BackupHandler, error) {
		attemptCount++
		if attemptCount == 1 {
			return failedHandler, nil
		}
		return successHandler, nil
	}

	onFail := func(_ context.Context) {
		failureCount++
	}

	onSuccess := func(_ context.Context, _ *models.BackupStats) error {
		successCount++
		return nil
	}

	onRetry := func() {
		retryCount++
	}

	handler := newRetryableBackupHandler(t.Context(), retry, retryableBackupCallbacks{
		Start: start, OnFail: onFail, OnSuccess: onSuccess, OnRetry: onRetry,
	}, slog.Default())
	err := handler.Wait(t.Context())

	require.NoError(t, err)
	assert.Equal(t, 1, failureCount)
	assert.Equal(t, 1, successCount)
	assert.Equal(t, 1, retryCount)
}

func TestStartRetryableBackup_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	ctrl := gomock.NewController(t)
	mockHandler := backupexecutor.NewMockBackupHandler(ctrl)

	waitCalled := make(chan struct{})
	mockHandler.EXPECT().Wait(gomock.Any()).DoAndReturn(func(_ context.Context) error {
		close(waitCalled)
		<-ctx.Done()
		return context.Canceled
	})

	successCount := 0
	failureCount := 0
	var mu sync.Mutex

	start := func(_ context.Context) (backupexecutor.BackupHandler, error) {
		return mockHandler, nil
	}

	onFail := func(_ context.Context) {
		mu.Lock()
		defer mu.Unlock()
		failureCount++
	}

	onSuccess := func(_ context.Context, _ *models.BackupStats) error {
		successCount++
		return nil
	}

	handler := newRetryableBackupHandler(ctx, retry, retryableBackupCallbacks{
		Start: start, OnFail: onFail, OnSuccess: onSuccess, OnRetry: func() {},
	}, slog.Default())

	<-waitCalled

	cancel()

	err := handler.Wait(context.WithoutCancel(t.Context())) // need to ensure cancel is coming from handler

	mu.Lock()
	defer mu.Unlock()

	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, failureCount)
	assert.Equal(t, 0, successCount)
}

func TestStartRetryableBackup_AllWaitAttemptsFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockHandler := backupexecutor.NewMockBackupHandler(ctrl)
	mockHandler.EXPECT().Wait(gomock.Any()).Return(errors.New("wait failed")).Times(3)

	successCount := 0
	failureCount := 0

	start := func(_ context.Context) (backupexecutor.BackupHandler, error) {
		return mockHandler, nil
	}

	onFail := func(_ context.Context) {
		failureCount++
	}

	onSuccess := func(_ context.Context, _ *models.BackupStats) error {
		successCount++
		return nil
	}

	handler := newRetryableBackupHandler(t.Context(), retry, retryableBackupCallbacks{
		Start: start, OnFail: onFail, OnSuccess: onSuccess, OnRetry: func() {},
	}, slog.Default())
	err := handler.Wait(t.Context())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed after 3 attempts")
	assert.Contains(t, err.Error(), "backup failed")
	assert.Equal(t, 3, failureCount)
	assert.Equal(t, 0, successCount)
}

func TestStartRetryableBackup_StartFails(t *testing.T) {
	successCount := 0
	failureCount := 0

	start := func(_ context.Context) (backupexecutor.BackupHandler, error) {
		return nil, errors.New("start failed")
	}

	onFail := func(_ context.Context) {
		failureCount++
	}

	onSuccess := func(_ context.Context, _ *models.BackupStats) error {
		successCount++
		return nil
	}

	handler := newRetryableBackupHandler(t.Context(), retry, retryableBackupCallbacks{
		Start: start, OnFail: onFail, OnSuccess: onSuccess, OnRetry: func() {},
	}, slog.Default())
	err := handler.Wait(t.Context())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start backup")
	assert.Equal(t, 0, failureCount)
	assert.Equal(t, 0, successCount)
}

func TestStartRetryableBackup_Cancel(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockHandler := backupexecutor.NewMockBackupHandler(ctrl)

	// The handler should wait until context is done, then return context.Canceled
	mockHandler.EXPECT().Wait(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
		select {
		case <-time.After(1 * time.Second):
			return errors.New("cancel was not called")
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	successCount := 0
	failureCount := 0

	start := func(_ context.Context) (backupexecutor.BackupHandler, error) {
		return mockHandler, nil
	}

	onFail := func(_ context.Context) {
		failureCount++
	}

	onSuccess := func(_ context.Context, _ *models.BackupStats) error {
		successCount++
		return nil
	}

	handler := newRetryableBackupHandler(t.Context(), retry, retryableBackupCallbacks{
		Start: start, OnFail: onFail, OnSuccess: onSuccess, OnRetry: func() {},
	}, slog.Default())
	var err error
	var wg sync.WaitGroup
	wg.Go(func() {
		err = handler.Wait(t.Context())
	})

	handler.Cancel()
	wg.Wait()

	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, failureCount)
	require.Equal(t, 0, successCount)
}

func TestStartRetryableBackup_ReleasesWaitContextOnCompletion(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockHandler := backupexecutor.NewMockBackupHandler(ctrl)

	// The context the inner handler waits under is this run's cancel handle.
	waitCtxCh := make(chan context.Context, 1)
	mockHandler.EXPECT().Wait(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
		waitCtxCh <- ctx
		return nil
	})
	mockHandler.EXPECT().GetStats().Return(models.NewBackupStats()).AnyTimes()

	handler := newRetryableBackupHandler(t.Context(), retry, retryableBackupCallbacks{
		Start:     func(context.Context) (backupexecutor.BackupHandler, error) { return mockHandler, nil },
		OnFail:    func(context.Context) {},
		OnSuccess: func(context.Context, *models.BackupStats) error { return nil },
		OnRetry:   func() {},
	}, slog.Default())

	require.NoError(t, handler.Wait(t.Context()))

	waitCtx := <-waitCtxCh

	// A finished run releases its wait context, so it stops being registered in the long-lived
	// parent it was derived from.
	select {
	case <-waitCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("the wait context was still live after the backup finished")
	}
}

func TestStartRetryableBackup_CancelStopsMetadataRetries(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockHandler := backupexecutor.NewMockBackupHandler(ctrl)

	// The data phase succeeds; the metadata write is what keeps failing.
	mockHandler.EXPECT().Wait(gomock.Any()).Return(nil)
	mockHandler.EXPECT().GetStats().Return(models.NewBackupStats()).AnyTimes()

	var onSuccessCalls, failureCount atomic.Int32
	firstCall := make(chan struct{})
	canceled := make(chan struct{})

	handler := newRetryableBackupHandler(t.Context(), retry, retryableBackupCallbacks{
		Start:  func(context.Context) (backupexecutor.BackupHandler, error) { return mockHandler, nil },
		OnFail: func(context.Context) { failureCount.Add(1) },
		OnSuccess: func(context.Context, *models.BackupStats) error {
			if onSuccessCalls.Add(1) == 1 {
				close(firstCall)
			}
			<-canceled // hold the first write open until the run has been canceled
			return errors.New("metadata write failed")
		},
		OnRetry: func() {},
	}, slog.Default())

	<-firstCall
	handler.Cancel()
	close(canceled)

	err := handler.Wait(t.Context())

	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, int32(1), onSuccessCalls.Load(),
		"Cancel must stop the metadata retry loop, not only the data phase")
	require.Equal(t, int32(1), failureCount.Load())
}

func TestRetryableBackupHandler_GetStats_GetMetrics(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	mockHandler := backupexecutor.NewMockBackupHandler(ctrl)
	stats := models.NewBackupStats()
	metrics := &models.Metrics{RecordsPerSecond: 42}

	mockHandler.EXPECT().Wait(gomock.Any()).Return(nil)
	mockHandler.EXPECT().GetStats().Return(stats).AnyTimes()
	mockHandler.EXPECT().GetMetrics().Return(metrics).AnyTimes()

	start := func(_ context.Context) (backupexecutor.BackupHandler, error) {
		return mockHandler, nil
	}
	onSuccess := func(_ context.Context, _ *models.BackupStats) error { return nil }

	handler := newRetryableBackupHandler(t.Context(), retry, retryableBackupCallbacks{
		Start: start, OnFail: func(context.Context) {}, OnSuccess: onSuccess, OnRetry: func() {},
	}, slog.Default())

	require.NoError(t, handler.Wait(t.Context()))
	assert.Equal(t, metrics, handler.GetMetrics())
	assert.Equal(t, stats, handler.GetStats())
}

func TestRetryableBackupHandler_GetStats_GetMetrics_BeforeStart(t *testing.T) {
	t.Parallel()

	h := &retryableBackupHandler{}
	assert.Nil(t, h.GetStats())
	assert.Nil(t, h.GetMetrics())
}

// waitStarted returns as soon as the pipeline is running, while the backup itself goes on.
func TestRetryableBackupHandler_WaitStarted_ReturnsOnceRunning(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockHandler := backupexecutor.NewMockBackupHandler(ctrl)
	mockHandler.EXPECT().Wait(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
		<-ctx.Done()

		return ctx.Err()
	})

	handler := newRetryableBackupHandler(t.Context(), retry, retryableBackupCallbacks{
		Start: func(_ context.Context) (backupexecutor.BackupHandler, error) {
			return mockHandler, nil
		},
		OnFail:    func(_ context.Context) {},
		OnSuccess: func(_ context.Context, _ *models.BackupStats) error { return nil },
		OnRetry:   func() {},
	}, slog.Default())
	defer handler.Cancel()

	require.NoError(t, handler.waitStarted(t.Context()))
}

// A run that never starts a pipeline - an unreachable cluster, a storage writer that cannot be
// created - ends the wait with the error that ended the run, instead of blocking until the
// caller's context is canceled, which for a scheduled backup is at shutdown.
func TestRetryableBackupHandler_WaitStarted_ReturnsStartError(t *testing.T) {
	handler := newRetryableBackupHandler(t.Context(), retry, retryableBackupCallbacks{
		Start: func(_ context.Context) (backupexecutor.BackupHandler, error) {
			return nil, errors.New("cluster unreachable")
		},
		OnFail:    func(_ context.Context) {},
		OnSuccess: func(_ context.Context, _ *models.BackupStats) error { return nil },
		OnRetry:   func() {},
	}, slog.Default())

	err := handler.waitStarted(t.Context())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cluster unreachable")
}

// A run that has already finished by the time the wait is reached is a started run.
func TestRetryableBackupHandler_WaitStarted_AcceptsFinishedRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockHandler := backupexecutor.NewMockBackupHandler(ctrl)
	mockHandler.EXPECT().Wait(gomock.Any()).Return(nil)
	mockHandler.EXPECT().GetStats().Return(models.NewBackupStats())

	handler := newRetryableBackupHandler(t.Context(), retry, retryableBackupCallbacks{
		Start: func(_ context.Context) (backupexecutor.BackupHandler, error) {
			return mockHandler, nil
		},
		OnFail:    func(_ context.Context) {},
		OnSuccess: func(_ context.Context, _ *models.BackupStats) error { return nil },
		OnRetry:   func() {},
	}, slog.Default())
	require.NoError(t, handler.Wait(t.Context()))

	require.NoError(t, handler.waitStarted(t.Context()))
}

// A backup that started and then failed still started: the failure belongs to Wait, so that the
// whole routine run is reported the same way whenever it fails.
func TestRetryableBackupHandler_WaitStarted_AcceptsRunThatFailedAfterStarting(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockHandler := backupexecutor.NewMockBackupHandler(ctrl)
	mockHandler.EXPECT().Wait(gomock.Any()).Return(errors.New("wait failed")).Times(3)

	handler := newRetryableBackupHandler(t.Context(), retry, retryableBackupCallbacks{
		Start: func(_ context.Context) (backupexecutor.BackupHandler, error) {
			return mockHandler, nil
		},
		OnFail:    func(_ context.Context) {},
		OnSuccess: func(_ context.Context, _ *models.BackupStats) error { return nil },
		OnRetry:   func() {},
	}, slog.Default())
	require.Error(t, handler.Wait(t.Context()))

	require.NoError(t, handler.waitStarted(t.Context()))
}

// The wait is bounded by the caller's context, not by the run.
func TestRetryableBackupHandler_WaitStarted_HonorsContext(t *testing.T) {
	handler := newRetryableBackupHandler(t.Context(), retry, retryableBackupCallbacks{
		Start: func(ctx context.Context) (backupexecutor.BackupHandler, error) {
			<-ctx.Done()

			return nil, ctx.Err()
		},
		OnFail:    func(_ context.Context) {},
		OnSuccess: func(_ context.Context, _ *models.BackupStats) error { return nil },
		OnRetry:   func() {},
	}, slog.Default())
	defer handler.Cancel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
	defer cancel()

	require.ErrorIs(t, handler.waitStarted(ctx), context.DeadlineExceeded)
}
