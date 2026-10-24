package try

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/aerospike/aerospike-client-go/v8/types"
	"github.com/aerospike/backup-go/errclass"
	"github.com/aerospike/backup-go/models"
)

// nonRetryableClasses are the backup-go error classes whose cause cannot change between
// attempts. The library documents the classification as part of its contract, so matching on
// these is stable in a way that matching on individual errors or messages is not.
var nonRetryableClasses = []error{
	errclass.ErrInvalidConfig,
	errclass.ErrCorruptData,
	errclass.ErrUnsupported,
	errclass.ErrNotFound,
}

// permanentResultCodes are the Aerospike result codes that report a malformed request rather
// than a passing condition of the cluster. The cluster answers a retry the same way, so the
// attempt budget and its back-off are spent for nothing. A filter expression the server cannot
// parse arrives here as PARAMETER_ERROR.
var permanentResultCodes = []types.ResultCode{
	types.PARAMETER_ERROR,
	types.INVALID_NAMESPACE,
	types.BIN_TYPE_ERROR,
}

// retryable reports whether another attempt could plausibly come out differently.
//
// Aerospike failures need the result code to tell the two apart: the class only says the
// cluster or its client reported the failure, which covers a timeout worth retrying and a
// malformed request that never will be. backup-go wraps the client's error rather than
// replacing it, so the code stays reachable here.
func retryable(err error) bool {
	for _, class := range nonRetryableClasses {
		if errors.Is(err, class) {
			return false
		}
	}

	var asErr *as.AerospikeError
	if errors.As(err, &asErr) && asErr.Matches(permanentResultCodes...) {
		return false
	}

	return true
}

// Retry runs f with retries according to policy.
// Pass a logger scoped with context (e.g. logger.With(slog.String("label", "backup"))).
// onRetry is invoked before each Retry (not on the final failed attempt).
//
// A failure is tried again unless retryable rules it out. Cancellation is judged from ctx and
// not from the error, because an operation may return context.Canceled for reasons of its own,
// such as a pipeline stopping its workers after one of them failed, and that is a failure to
// retry like any other. Only when ctx itself is done has the caller withdrawn; then the loop
// stops at once, before any further attempt or back-off, and returns the last failure joined
// with ctx.Err() unless the failure already carries it.
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

		if !retryable(lastErr) {
			logger.Info("Non-retryable error encountered, aborting without Retry", attr.Error(lastErr))
			return lastErr
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
