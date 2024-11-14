package service

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var retry = NewRetryService(models.RetryPolicy{
	MaxRetries:  2,
	BaseTimeout: 100 * time.Millisecond,
	Multiplier:  1,
}, slog.Default())

func TestStartRetryableBackup_SuccessfulFirstAttempt(t *testing.T) {
	ctx := context.Background()
	mockHandler := &mockBackupHandler{}
	stats := &models.BackupStats{}
	mockHandler.On("Wait", mock.Anything).Return(nil)
	mockHandler.On("GetStats").Return(stats)

	successCount := 0
	failureCount := 0

	start := func(_ context.Context) (BackupHandler, error) {
		return mockHandler, nil
	}

	onFail := func(_ context.Context) {
		failureCount++
	}

	onSuccess := func(_ context.Context, _ *models.BackupStats) error {
		successCount++
		return nil
	}

	handler := startRetryableBackup(ctx, retry, start, onFail, onSuccess)
	err := handler.Wait(ctx)

	assert.NoError(t, err)
	assert.Equal(t, 0, failureCount)
	assert.Equal(t, 1, successCount)
	mockHandler.AssertExpectations(t)
}

func TestStartRetryableBackup_WaitFailsThenSucceeds(t *testing.T) {
	ctx := context.Background()
	failedHandler := &mockBackupHandler{}
	successHandler := &mockBackupHandler{}
	stats := &models.BackupStats{}

	failedHandler.On("Wait", mock.Anything).Return(errors.New("wait failed"))
	successHandler.On("Wait", mock.Anything).Return(nil)
	successHandler.On("GetStats").Return(stats)

	attemptCount := 0
	successCount := 0
	failureCount := 0

	start := func(_ context.Context) (BackupHandler, error) {
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

	handler := startRetryableBackup(ctx, retry, start, onFail, onSuccess)
	err := handler.Wait(ctx)

	assert.NoError(t, err)
	assert.Equal(t, 1, failureCount)
	assert.Equal(t, 1, successCount)
	failedHandler.AssertExpectations(t)
	successHandler.AssertExpectations(t)
}

func TestStartRetryableBackup_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	mockHandler := &mockBackupHandler{}

	waitCalled := make(chan struct{})
	mockHandler.On("Wait", mock.Anything).Run(func(_ mock.Arguments) {
		close(waitCalled)
		<-ctx.Done()
	}).Return(context.Canceled)

	successCount := 0
	failureCount := 0
	var mu sync.Mutex

	start := func(_ context.Context) (BackupHandler, error) {
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

	handler := startRetryableBackup(ctx, retry, start, onFail, onSuccess)

	<-waitCalled

	cancel()

	err := handler.Wait(ctx)

	mu.Lock()
	defer mu.Unlock()

	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 0, failureCount)
	assert.Equal(t, 0, successCount)
	mockHandler.AssertExpectations(t)
}

func TestStartRetryableBackup_AllWaitAttemptsFail(t *testing.T) {
	ctx := context.Background()
	mockHandler := &mockBackupHandler{}
	mockHandler.On("Wait", mock.Anything).Return(errors.New("wait failed"))

	successCount := 0
	failureCount := 0

	start := func(_ context.Context) (BackupHandler, error) {
		return mockHandler, nil
	}

	onFail := func(_ context.Context) {
		failureCount++
	}

	onSuccess := func(_ context.Context, _ *models.BackupStats) error {
		successCount++
		return nil
	}

	handler := startRetryableBackup(ctx, retry, start, onFail, onSuccess)
	err := handler.Wait(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "backup failed after 3 attempts")
	assert.Equal(t, 3, failureCount)
	assert.Equal(t, 0, successCount)
	mockHandler.AssertExpectations(t)
}

func TestStartRetryableBackup_StartFails(t *testing.T) {
	ctx := context.Background()
	successCount := 0
	failureCount := 0

	start := func(_ context.Context) (BackupHandler, error) {
		return nil, errors.New("start failed")
	}

	onFail := func(_ context.Context) {
		failureCount++
	}

	onSuccess := func(_ context.Context, _ *models.BackupStats) error {
		successCount++
		return nil
	}

	handler := startRetryableBackup(ctx, retry, start, onFail, onSuccess)
	err := handler.Wait(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start backup")
	assert.Equal(t, 0, failureCount)
	assert.Equal(t, 0, successCount)
}
