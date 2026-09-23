package try

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/aerospike/aerospike-client-go/v8/types"
	"github.com/aerospike/backup-go/errclass"
	"github.com/aerospike/backup-go/models"
	"github.com/aerospike/backup-go/pkg/asinfo"
	"github.com/stretchr/testify/require"
)

// asFailure builds a failure shaped the way backup-go reports one from the Aerospike client:
// the class marks where it came from, and the client's own error stays in the chain.
func asFailure(code types.ResultCode) error {
	return fmt.Errorf("%w: failed to read record: %w",
		errclass.ErrAerospike, &as.AerospikeError{ResultCode: code})
}

func Test_retry_classification(t *testing.T) {
	tests := map[string]struct {
		err          error
		wantAttempts int
	}{
		"an unclassified failure is retried": {
			err:          errors.New("connection reset by peer"),
			wantAttempts: 3,
		},
		"a timeout from the cluster is retried": {
			err:          asFailure(types.TIMEOUT),
			wantAttempts: 3,
		},
		"a malformed request is not retried": {
			err:          asFailure(types.PARAMETER_ERROR),
			wantAttempts: 1,
		},
		"an unknown namespace is not retried": {
			err:          asFailure(types.INVALID_NAMESPACE),
			wantAttempts: 1,
		},
		"invalid config is not retried": {
			err:          fmt.Errorf("wrapped: %w", errclass.ErrInvalidConfig),
			wantAttempts: 1,
		},
		"corrupt data is not retried": {
			err:          fmt.Errorf("wrapped: %w", errclass.ErrCorruptData),
			wantAttempts: 1,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			attempts := 0

			err := Retry(t.Context(), testRetryPolicy, slog.Default(), func() error {
				attempts++
				return tt.err
			}, func() {})

			require.Error(t, err)
			require.Equal(t, tt.wantAttempts, attempts)
		})
	}
}

var testRetryPolicy = models.RetryPolicy{
	MaxRetries:  2,
	BaseTimeout: 100 * time.Millisecond,
	Multiplier:  1,
}

func Test_timer(t *testing.T) {
	counterLock := sync.Mutex{}
	retryCounter := 2
	err := Retry(t.Context(), testRetryPolicy, slog.Default(), func() error {
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
	_ = Retry(t.Context(), testRetryPolicy, slog.Default(), func() error {
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
	_ = Retry(t.Context(), testRetryPolicy, slog.Default(), f, func() {})
	_ = Retry(t.Context(), testRetryPolicy, slog.Default(), f, func() {})

	counterLock.Lock()
	defer counterLock.Unlock()
	require.Equal(t, 0, retryCounter)
}

func Test_retry_attempts_expected_count(t *testing.T) {
	attempts := 0
	expectedAttempts := 3 // MaxRetries=2 + 1 initial attempt

	err := Retry(t.Context(), testRetryPolicy, slog.Default(), func() error {
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

	err := Retry(t.Context(), policy, slog.Default(), func() error {
		attempts++
		return fmt.Errorf("wrapped: %w", asinfo.ErrNoNode)
	}, func() { onRetryCalls++ })

	require.Error(t, err)
	require.ErrorIs(t, err, asinfo.ErrNoNode)
	require.Equal(t, 1, attempts, "should attempt only once for non-retryable error")
	require.Equal(t, 0, onRetryCalls, "onRetry should not be called for non-retryable error")
}

func Test_canceled_context_interrupts_backoff(t *testing.T) {
	policy := models.RetryPolicy{
		MaxRetries:  5,
		BaseTimeout: time.Minute,
		Multiplier:  1,
	}
	ctx, cancel := context.WithCancel(t.Context())
	attempts := 0
	start := time.Now()

	err := Retry(ctx, policy, slog.Default(), func() error {
		attempts++
		cancel()
		return errors.New("mock error")
	}, func() {})

	require.ErrorIs(t, err, context.Canceled)
	require.ErrorContains(t, err, "mock error")
	require.Equal(t, 1, attempts, "no attempt should follow a canceled context")
	require.Less(t, time.Since(start), policy.BaseTimeout, "back-off outlived the context")
}

func Test_own_cancellation_error_is_retried(t *testing.T) {
	attempts := 0

	err := Retry(t.Context(), testRetryPolicy, slog.Default(), func() error {
		attempts++
		// The operation canceled itself; the caller's context is still live.
		return fmt.Errorf("pipeline stopped: %w", context.Canceled)
	}, func() {})

	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, int(testRetryPolicy.MaxRetries)+1, attempts,
		"a Canceled error from f alone must not stop the loop")
}

func Test_canceled_context_stops_before_next_attempt(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	attempts := 0

	err := Retry(ctx, testRetryPolicy, slog.Default(), func() error {
		attempts++
		cancel()
		return fmt.Errorf("wait: %w", context.Canceled)
	}, func() {})

	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, attempts, "no attempt should follow a canceled context")
	require.Equal(t, 1, strings.Count(err.Error(), context.Canceled.Error()),
		"a failure that already carries the context error is not joined with it again")
}
