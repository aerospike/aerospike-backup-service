package service

import (
	"context"
	"errors"
	"fmt"
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

var retry = newRetryExecutor(models.RetryPolicy{
	MaxRetries:  2,
	BaseTimeout: time.Millisecond,
	Multiplier:  1,
}, slog.Default())

func TestStartRetryableBackup_SuccessfulFirstAttempt(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockHandler := backupexecutor.NewMockBackupHandler(ctrl)
	stats := models.NewBackupStats()
	mockHandler.EXPECT().Wait(gomock.Any()).Return(nil)
	mockHandler.EXPECT().GetStats().Return(stats)

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

	handler := newRetryableBackupHandler(ctx, retry, start, onFail, onSuccess)
	err := handler.Wait(ctx)

	assert.NoError(t, err)
	assert.Equal(t, 0, failureCount)
	assert.Equal(t, 1, successCount)

}

func TestStartRetryableBackup_WaitFailsThenSucceeds(t *testing.T) {
	ctx := context.Background()
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

	handler := newRetryableBackupHandler(ctx, retry, start, onFail, onSuccess)
	err := handler.Wait(ctx)

	assert.NoError(t, err)
	assert.Equal(t, 1, failureCount)
	assert.Equal(t, 1, successCount)
}

func TestStartRetryableBackup_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
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

	handler := newRetryableBackupHandler(ctx, retry, start, onFail, onSuccess)

	<-waitCalled

	cancel()

	err := handler.Wait(ctx)

	mu.Lock()
	defer mu.Unlock()

	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 0, failureCount)
	assert.Equal(t, 0, successCount)
}

func TestStartRetryableBackup_AllWaitAttemptsFail(t *testing.T) {
	ctx := context.Background()
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

	handler := newRetryableBackupHandler(ctx, retry, start, onFail, onSuccess)
	err := handler.Wait(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "backup failed after 3 attempts")
	assert.Equal(t, 3, failureCount)
	assert.Equal(t, 0, successCount)
}

func TestStartRetryableBackup_StartFails(t *testing.T) {
	ctx := context.Background()
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

	handler := newRetryableBackupHandler(ctx, retry, start, onFail, onSuccess)
	err := handler.Wait(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start backup")
	assert.Equal(t, 0, failureCount)
	assert.Equal(t, 0, successCount)
}

func TestStartRetryableBackup_Cancel(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockHandler := backupexecutor.NewMockBackupHandler(ctrl)

	// The handler should wait until context is done, then return context.Canceled
	mockHandler.EXPECT().Wait(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
		select {
		case <-time.After(1 * time.Second):
			return fmt.Errorf("cancel was not called")
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

	handler := newRetryableBackupHandler(ctx, retry, start, onFail, onSuccess)
	var err error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		err = handler.Wait(ctx)
		wg.Done()
	}()

	handler.Cancel()
	wg.Wait()

	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, failureCount)
	require.Equal(t, 0, successCount)
}
