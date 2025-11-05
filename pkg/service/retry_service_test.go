package service

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/require"
)

var r = newRetryExecutor(models.RetryPolicy{
	MaxRetries:  2,
	BaseTimeout: 100 * time.Millisecond,
	Multiplier:  1,
}, slog.Default())

func Test_timer(t *testing.T) {
	counterLock := sync.Mutex{}
	retryCounter := 2
	err := r.run("test", func() error {
		counterLock.Lock()
		defer counterLock.Unlock()
		if retryCounter > 0 {
			retryCounter--
			return errors.New("mock error")
		}
		return nil
	}, func() {})
	require.NoError(t, err)

	time.Sleep(1 * time.Second)
	counterLock.Lock()
	defer counterLock.Unlock()
	if retryCounter != 0 {
		t.Errorf("Expected retryCounter 0, got %d", retryCounter)
	}
}

func Test_timer_expires(t *testing.T) {
	counterLock := sync.Mutex{}
	retryCounter := 0
	const attempts = 3
	_ = r.run("test", func() error {
		counterLock.Lock()
		defer counterLock.Unlock()
		retryCounter++
		return errors.New("mock error")
	}, func() {})

	time.Sleep(1 * time.Second)
	counterLock.Lock()
	defer counterLock.Unlock()
	if retryCounter != attempts {
		t.Errorf("Expected retryCounter %d, got %d", attempts, retryCounter)
	}
}

func Test_timerRunTwice(t *testing.T) {
	counterLock := sync.Mutex{}
	retryCounter := 3
	f := func() error {
		counterLock.Lock()
		defer counterLock.Unlock()
		if retryCounter > 0 {
			retryCounter--
			return errors.New("mock error")
		}
		return nil
	}
	_ = r.run("test", f, func() {})
	_ = r.run("test", f, func() {})

	time.Sleep(1 * time.Second)
	counterLock.Lock()
	defer counterLock.Unlock()
	if retryCounter != 0 {
		t.Errorf("Expected retryCounter 0, got %d", retryCounter)
	}
}

func Test_retry_attempts_expected_count(t *testing.T) {
	attempts := 0
	expectedAttempts := 3 // MaxRetries=2 + 1 initial attempt

	err := r.run("retry-test", func() error {
		attempts++
		return errors.New("still failing")
	}, func() {})

	require.Error(t, err)
	require.Equal(t, expectedAttempts, attempts, "Function was not retried the expected number of times")
}

func Test_non_retryable_error_stops_retries(t *testing.T) {
	sentinel := errors.New("permanent failure")

	attempts := 0
	onRetryCalls := 0

	re := newRetryExecutor(models.RetryPolicy{
		MaxRetries:  5,
		BaseTimeout: time.Millisecond,
		Multiplier:  1,
	}, slog.Default(), sentinel)

	err := re.run("non-retryable", func() error {
		attempts++
		return fmt.Errorf("wrapped: %w", sentinel)
	}, func() { onRetryCalls++ })

	require.Error(t, err)
	require.ErrorIs(t, err, sentinel)
	require.Equal(t, 1, attempts, "should attempt only once for non-retryable error")
	require.Equal(t, 0, onRetryCalls, "onRetry should not be called for non-retryable error")
}
