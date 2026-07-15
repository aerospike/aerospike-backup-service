package service

import (
	"context"
	"errors"
	"log/slog"
	"sync"
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
	defer ctrl.Finish()

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
	defer ctrl.Finish()

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
	defer ctrl.Finish()

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
	defer ctrl.Finish()

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
	defer ctrl.Finish()

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
