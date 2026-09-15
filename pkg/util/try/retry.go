package try

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/backup-go/models"
	"github.com/aerospike/backup-go/pkg/asinfo"
)

var nonRetryableErrors = []error{asinfo.ErrNoNode}

// Retry runs f with retries according to policy.
// Pass a logger scoped with context (e.g. logger.With(slog.String("label", "backup"))).
// onRetry is invoked before each Retry (not on the final failed attempt).
//
// Whether to try again is decided by ctx, never by the shape of the error: an operation may
// return context.Canceled for reasons of its own, such as a pipeline stopping its workers after
// one of them failed, and that is a failure to retry like any other. Only when ctx itself is done
// has the caller withdrawn; then the loop stops at once, before any further attempt or back-off,
// and returns the last failure joined with ctx.Err() unless the failure already carries it.
func Retry(
	ctx context.Context,
	policy models.RetryPolicy,
	logger *slog.Logger,
	f func() error,
	onRetry func(),
) error {
	var (
		lastErr       error
		retryInterval = policy.BaseTimeout
		totalAttempts = policy.MaxRetries + 1
	)

	for attempt := uint(1); attempt <= totalAttempts; attempt++ {
		lastErr = f()
		if lastErr == nil {
			return nil
		}

		if ctxErr := ctx.Err(); ctxErr != nil {
			logger.Info("Retry aborted, context done", attr.Error(lastErr))
			return joinContextErr(lastErr, ctxErr)
		}

		for _, nre := range nonRetryableErrors {
			if nre != nil && errors.Is(lastErr, nre) {
				logger.Info("Non-retryable error encountered, aborting without Retry", attr.Error(lastErr))
				return lastErr
			}
		}

		if attempt < totalAttempts {
			onRetry()
			logger.Info("Execution failed, retrying...",
				slog.Any("attempt", attempt),
				slog.Any("maxAttempts", policy.MaxRetries),
				slog.Any("retryInterval", retryInterval),
				attr.Error(lastErr))
			select {
			case <-time.After(retryInterval):
			case <-ctx.Done():
				logger.Info("Retry aborted, context done", attr.Error(lastErr))
				return joinContextErr(lastErr, ctx.Err())
			}

			retryInterval = nextRetryInterval(retryInterval, policy.Multiplier)
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", totalAttempts, lastErr)
}

// nextRetryInterval grows the back-off by multiplier and saturates at the largest Duration. An
// overflowing float-to-int conversion is implementation-defined and wraps negative on amd64, which
// would turn the back-off into a hot loop.
func nextRetryInterval(current time.Duration, multiplier float64) time.Duration {
	next := float64(current) * multiplier
	if next >= float64(math.MaxInt64) {
		return time.Duration(math.MaxInt64)
	}

	return time.Duration(next)
}

// joinContextErr attaches the reason the context ended to the failure it interrupted, unless
// the failure already reports that reason, so callers see "context canceled" once, not twice.
func joinContextErr(err, ctxErr error) error {
	if errors.Is(err, ctxErr) {
		return err
	}

	return errors.Join(err, ctxErr)
}