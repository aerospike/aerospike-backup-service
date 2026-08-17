package try

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/aerospike/backup-go/models"
	"github.com/aerospike/backup-go/pkg/asinfo"
	"github.com/stretchr/testify/require"
)

var testRetryPolicy = models.RetryPolicy{
	MaxRetries:  2,
	BaseTimeout: 100 * time.Millisecond,
	Multiplier:  1,
}

func Test_timer(t *testing.T) {
	counterLock := sync.Mutex{}
	retryCounter := 2
	err := Retry(testRetryPolicy, slog.Default(), func() error {
		counterLock.Lock()
		defer counterLock.Unlock()
		if retryCounter > 0 {
			retryCounter--
			return errors.New("mock error")
		}
		return nil
	}, func() {})
	require.NoError(t, err)

	counterLock.Lock()
	defer counterLock.Unlock()
	require.Equal(t, 0, retryCounter)
}

func Test_timer_expires(t *testing.T) {
	counterLock := sync.Mutex{}
	retryCounter := 0
	const attempts = 3
	_ = Retry(testRetryPolicy, slog.Default(), func() error {
		counterLock.Lock()
		defer counterLock.Unlock()
		retryCounter++
		return errors.New("mock error")
	}, func() {})

	counterLock.Lock()
	defer counterLock.Unlock()
	require.Equal(t, attempts, retryCounter)
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
	_ = Retry(testRetryPolicy, slog.Default(), f, func() {})
	_ = Retry(testRetryPolicy, slog.Default(), f, func() {})

	counterLock.Lock()
	defer counterLock.Unlock()
	require.Equal(t, 0, retryCounter)
}

func Test_retry_attempts_expected_count(t *testing.T) {
	attempts := 0
	expectedAttempts := 3 // MaxRetries=2 + 1 initial attempt

	err := Retry(testRetryPolicy, slog.Default(), func() error {
		attempts++
		return errors.New("still failing")
	}, func() {})

	require.Error(t, err)
	require.Equal(t, expectedAttempts, attempts, "Function was not retried the expected number of times")
}

func Test_non_retryable_error_stops_retries(t *testing.T) {
	attempts := 0
	onRetryCalls := 0

	policy := models.RetryPolicy{
		MaxRetries:  5,
		BaseTimeout: time.Millisecond,
		Multiplier:  1,
	}

	err := Retry(policy, slog.Default(), func() error {
		attempts++
		return fmt.Errorf("wrapped: %w", asinfo.ErrNoNode)
	}, func() { onRetryCalls++ })

	require.Error(t, err)
	require.ErrorIs(t, err, asinfo.ErrNoNode)
	require.Equal(t, 1, attempts, "should attempt only once for non-retryable error")
	require.Equal(t, 0, onRetryCalls, "onRetry should not be called for non-retryable error")
}
